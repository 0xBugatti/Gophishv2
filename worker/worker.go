package worker

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/gophish/gophish/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Worker is an interface that defines the operations needed for a background worker
type Worker interface {
	Start()
	LaunchCampaign(c models.Campaign)
	SendTestEmail(s *models.EmailRequest) error
	SendTestSMS(s *models.SMSRequest) error
}

// DefaultWorker is the background worker that handles watching for new campaigns and sending emails and SMS messages appropriately.
type DefaultWorker struct {
	mailer    mailer.Mailer
	smsMailer *mailer.SMSWorker
}

// New creates a new worker object to handle the creation of campaigns
func New(options ...func(Worker) error) (Worker, error) {
	defaultMailer := mailer.NewMailWorker()
	defaultSMSMailer := mailer.NewSMSWorker()
	w := &DefaultWorker{
		mailer:    defaultMailer,
		smsMailer: defaultSMSMailer,
	}
	for _, opt := range options {
		if err := opt(w); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// WithMailer sets the mailer for a given worker.
// By default, workers use a standard, default mailworker.
func WithMailer(m mailer.Mailer) func(*DefaultWorker) error {
	return func(w *DefaultWorker) error {
		w.mailer = m
		return nil
	}
}

// processCampaigns loads maillogs scheduled to be sent before the provided
// time and sends them to the mailer.
func (w *DefaultWorker) processCampaigns(t time.Time) error {
	ms, err := models.GetQueuedMailLogs(t.UTC())
	if err != nil {
		log.Error(err)
		return err
	}
	// Lock the MailLogs (they will be unlocked after processing)
	err = models.LockMailLogs(ms, true)
	if err != nil {
		return err
	}
	campaignCache := make(map[int64]models.Campaign)
	// We'll group the maillogs by campaign ID to (roughly) group
	// them by sending profile. This lets the mailer re-use the Sender
	// instead of having to re-connect to the SMTP server for every
	// email.
	msg := make(map[int64][]mailer.Mail)
	for _, m := range ms {
		// We cache the campaign here to greatly reduce the time it takes to
		// generate the message (ref #1726)
		c, ok := campaignCache[m.CampaignId]
		if !ok {
			c, err = models.GetCampaignMailContext(m.CampaignId, m.UserId)
			if err != nil {
				return err
			}
			campaignCache[c.Id] = c
		}
		m.CacheCampaign(&c)
		msg[m.CampaignId] = append(msg[m.CampaignId], m)
	}

	// Next, we process each group of maillogs in parallel
	for cid, msc := range msg {
		go func(cid int64, msc []mailer.Mail) {
			c := campaignCache[cid]
			if c.Status == models.CampaignQueued {
				err := c.UpdateStatus(models.CampaignInProgress)
				if err != nil {
					log.Error(err)
					return
				}
			}
			log.WithFields(logrus.Fields{
				"num_emails": len(msc),
			}).Info("Sending emails to mailer for processing")
			w.mailer.Queue(msc)
		}(cid, msc)
	}
	return nil
}

// Start launches the worker to poll the database every minute for any pending maillogs
// and smslogs that need to be processed.
func (w *DefaultWorker) Start() {
	log.Info("Background Worker Started Successfully - Waiting for Campaigns")
	ctx := context.Background()
	go w.mailer.Start(ctx)
	go w.smsMailer.Start(ctx)
	for t := range time.Tick(1 * time.Minute) {
		// Auto-complete campaigns whose EndTime has passed (4.5).
		expired, err := models.GetExpiredCampaigns(t.UTC())
		if err != nil {
			log.Error(err)
		}
		for _, c := range expired {
			log.WithFields(logrus.Fields{
				"campaign_id": c.Id,
			}).Info("Auto-completing expired campaign")
			if err := models.CompleteCampaign(c.Id, c.UserId); err != nil {
				log.Error(err)
			}
		}

		err = w.processCampaigns(t)
		if err != nil {
			log.Error(err)
			continue
		}

		err = w.processSMSCampaigns(t)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}

// processSMSCampaigns loads smslogs scheduled to be sent before the provided
// time and sends them to the SMS mailer.
func (w *DefaultWorker) processSMSCampaigns(t time.Time) error {
	ss, err := models.GetQueuedSMSLogs(t.UTC())
	if err != nil {
		log.Error(err)
		return err
	}
	// Lock the SMSLogs (they will be unlocked after processing)
	err = models.LockSMSLogs(ss, true)
	if err != nil {
		return err
	}
	campaignCache := make(map[int64]models.Campaign)
	// We'll group the smslogs by campaign ID to (roughly) group
	// them by sending profile. This lets the mailer re-use the Sender
	// instead of having to re-connect to the SMS provider for every
	// message.
	msg := make(map[int64][]mailer.SMSMail)
	for _, s := range ss {
		// We cache the campaign here to greatly reduce the time it takes to
		// generate the message
		c, ok := campaignCache[s.CampaignId]
		if !ok {
			c, err = models.GetCampaignSMSContext(s.CampaignId, s.UserId)
			if err != nil {
				return err
			}
			campaignCache[c.Id] = c
		}
		s.CacheCampaign(&c)
		msg[s.CampaignId] = append(msg[s.CampaignId], s)
	}

	// Next, we process each group of smslogs in parallel
	for cid, ssc := range msg {
		go func(cid int64, ssc []mailer.SMSMail) {
			c := campaignCache[cid]
			if c.Status == models.CampaignQueued {
				err := c.UpdateStatus(models.CampaignInProgress)
				if err != nil {
					log.Error(err)
					return
				}
			}
			log.WithFields(logrus.Fields{
				"num_sms": len(ssc),
			}).Info("Sending SMS messages to SMS mailer for processing")
			w.smsMailer.Queue(ssc)
		}(cid, ssc)
	}
	return nil
}

// LaunchCampaign starts a campaign
func (w *DefaultWorker) LaunchCampaign(c models.Campaign) {
	// Handle different campaign types
	if c.Type == "sms" {
		w.launchSMSCampaign(c)
	} else {
		w.launchEmailCampaign(c)
	}
}

// shuffleMailLogs performs a Fisher-Yates shuffle on a slice of mail log
// pointers using crypto/rand for an unbiased, unpredictable send order.
func shuffleMailLogs(ms []*models.MailLog) {
	for i := len(ms) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			break
		}
		j := int(jBig.Int64())
		ms[i], ms[j] = ms[j], ms[i]
	}
}

// launchEmailCampaign starts an email campaign
func (w *DefaultWorker) launchEmailCampaign(c models.Campaign) {
	ms, err := models.GetMailLogsByCampaign(c.Id)
	if err != nil {
		log.Error(err)
		return
	}
	models.LockMailLogs(ms, true)

	// Randomize send order if configured (2.5)
	if c.RandomizeSendOrder {
		shuffleMailLogs(ms)
	}

	// Build a rate limiter if the SMTP profile specifies a send_rate (2.2).
	// A per-minute rate is converted to events-per-second for the token bucket.
	var limiter *rate.Limiter
	if c.SMTP.SendRate > 0 {
		rps := rate.Limit(float64(c.SMTP.SendRate) / 60.0)
		limiter = rate.NewLimiter(rps, 1)
	}

	// This is required since you cannot pass a slice of values
	// that implements an interface as a slice of that interface.
	mailEntries := []mailer.Mail{}
	currentTime := time.Now().UTC()
	campaignMailCtx, err := models.GetCampaignMailContext(c.Id, c.UserId)
	if err != nil {
		log.Error(err)
		return
	}
	ctx := context.Background()
	for _, m := range ms {
		// Only send the emails scheduled to be sent for the past minute to
		// respect the campaign scheduling options
		if m.SendDate.After(currentTime) {
			m.Unlock()
			continue
		}
		// A/B test: if template variants are configured, pick a weighted-random
		// template for this target and swap it into the cached campaign (3.8).
		variantCtx := campaignMailCtx
		if tid := c.PickTemplateVariant(); tid > 0 && tid != campaignMailCtx.TemplateId {
			t, tErr := models.GetTemplateById(tid)
			if tErr == nil {
				variantCtx.Template = t
				variantCtx.TemplateId = tid
			}
		}
		err = m.CacheCampaign(&variantCtx)
		if err != nil {
			log.Error(err)
			return
		}
		// Apply send_interval_ms delay between dispatches if set (2.2)
		if c.SendIntervalMs > 0 && len(mailEntries) > 0 {
			time.Sleep(time.Duration(c.SendIntervalMs) * time.Millisecond)
		}
		// Apply token-bucket rate limiter if send_rate is configured (2.2)
		if limiter != nil {
			if waitErr := limiter.Wait(ctx); waitErr != nil {
				log.Error(waitErr)
			}
		}
		mailEntries = append(mailEntries, m)
	}
	// If send_concurrency > 1, split entries into chunks and queue each
	// concurrently so multiple SMTP connections are used in parallel (2.4).
	concurrency := c.SendConcurrency
	if concurrency < 2 || len(mailEntries) < 2 {
		w.mailer.Queue(mailEntries)
	} else {
		chunkSize := (len(mailEntries) + concurrency - 1) / concurrency
		var wg sync.WaitGroup
		for i := 0; i < len(mailEntries); i += chunkSize {
			end := i + chunkSize
			if end > len(mailEntries) {
				end = len(mailEntries)
			}
			chunk := mailEntries[i:end]
			wg.Add(1)
			go func(c []mailer.Mail) {
				defer wg.Done()
				w.mailer.Queue(c)
			}(chunk)
		}
		wg.Wait()
	}
}

// launchSMSCampaign starts an SMS campaign
func (w *DefaultWorker) launchSMSCampaign(c models.Campaign) {
	ss, err := models.GetSMSLogsByCampaign(c.Id)
	if err != nil {
		log.Error(err)
		return
	}
	models.LockSMSLogs(ss, true)
	// This is required since you cannot pass a slice of values
	// that implements an interface as a slice of that interface.
	smsEntries := []mailer.SMSMail{}
	currentTime := time.Now().UTC()
	campaignSMSCtx, err := models.GetCampaignSMSContext(c.Id, c.UserId)
	if err != nil {
		log.Error(err)
		return
	}
	for _, s := range ss {
		// Only send the SMS messages scheduled to be sent for the past minute to
		// respect the campaign scheduling options
		if s.SendDate.After(currentTime) {
			s.Unlock()
			continue
		}
		err = s.CacheCampaign(&campaignSMSCtx)
		if err != nil {
			log.Error(err)
			return
		}
		smsEntries = append(smsEntries, s)
	}
	w.smsMailer.Queue(smsEntries)
}

// SendTestEmail sends a test email
func (w *DefaultWorker) SendTestEmail(s *models.EmailRequest) error {
	go func() {
		ms := []mailer.Mail{s}
		w.mailer.Queue(ms)
	}()
	return <-s.ErrorChan
}

// SendTestSMS sends a test SMS
func (w *DefaultWorker) SendTestSMS(s *models.SMSRequest) error {
	go func() {
		ms := []mailer.SMSMail{s}
		w.smsMailer.Queue(ms)
	}()
	return <-s.ErrorChan
}
