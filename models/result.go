package models

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"strconv"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/webhook"
	"github.com/jinzhu/gorm"
	"github.com/oschwald/maxminddb-golang"
)

type mmCity struct {
	GeoPoint mmGeoPoint `maxminddb:"location"`
}

type mmGeoPoint struct {
	Latitude  float64 `maxminddb:"latitude"`
	Longitude float64 `maxminddb:"longitude"`
}

// Result contains the fields for a result object,
// which is a representation of a target in a campaign.
type Result struct {
	Id           int64     `json:"-"`
	CampaignId   int64     `json:"-"`
	UserId       int64     `json:"-"`
	RId          string    `json:"id"`
	Status       string    `json:"status" sql:"not null"`
	IP           string    `json:"ip"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	SendDate     time.Time `json:"send_date"`
	Reported     bool      `json:"reported" sql:"not null"`
	ModifiedDate time.Time `json:"modified_date"`
	SMSTarget    bool      `json:"sms_target" sql:"default:false"`
	BaseRecipient
}

func (r *Result) createEvent(status string, details interface{}) (*Event, error) {
	e := &Event{Email: r.Email, Message: status}
	if details != nil {
		dj, err := json.Marshal(details)
		if err != nil {
			return nil, err
		}
		e.Details = string(dj)
	}
	if err := AddEvent(e, r.CampaignId); err != nil {
		return nil, err
	}
	return e, nil
}

// updateResultAtomic saves a result status+modified_date in a transaction that
// also inserts the event row. If the event insert fails the result row is NOT
// updated, keeping them consistent (fix for 2.1 — previously callers ignored
// the error from createEvent so divergence was silently possible).
func (r *Result) updateResultAtomic(status string, details interface{}, updateFn func(*Event) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	e := &Event{Email: r.Email, Message: status}
	if details != nil {
		dj, err := json.Marshal(details)
		if err != nil {
			tx.Rollback()
			return err
		}
		e.Details = string(dj)
	}
	e.CampaignId = r.CampaignId
	e.Time = time.Now().UTC()
	if err := tx.Save(e).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := updateFn(e); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Save(r).Error; err != nil {
		tx.Rollback()
		return err
	}
	// Fire webhooks outside the transaction so a webhook failure doesn't
	// roll back the DB write.
	if err := tx.Commit().Error; err != nil {
		return err
	}
	go fireEventWebhooks(e)
	return nil
}

// fireEventWebhooks dispatches enriched webhooks for an event asynchronously (6.9).
func fireEventWebhooks(e *Event) {
	whs, err := GetActiveWebhooks()
	if err != nil {
		log.Errorf("error getting active webhooks: %v", err)
		return
	}
	payload := enrichEventForWebhook(e)
	for _, wh := range whs {
		webhook.SendAll([]webhook.EndPoint{{URL: wh.URL, Secret: wh.Secret}}, payload)
	}
}

// HandleEmailSent updates a Result to indicate that the email has been
// successfully sent to the remote SMTP server
func (r *Result) HandleEmailSent() error {
	event, err := r.createEvent(EventSent, nil)
	if err != nil {
		return err
	}
	r.SendDate = event.Time
	r.Status = EventSent
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailError updates a Result to indicate that there was an error when
// attempting to send the email to the remote SMTP server.
func (r *Result) HandleEmailError(err error) error {
	event, err := r.createEvent(EventSendingError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = Error
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailBackoff updates a Result to indicate that the email received a
// temporary error and needs to be retried
func (r *Result) HandleEmailBackoff(err error, sendDate time.Time) error {
	event, err := r.createEvent(EventSendingError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = StatusRetry
	r.SendDate = sendDate
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailOpened updates a Result in the case where the recipient opened the
// email.
func (r *Result) HandleEmailOpened(details EventDetails) error {
	// Don't update the status if the user already clicked the link
	// or submitted data to the campaign
	if r.Status == EventClicked || r.Status == EventDataSubmit {
		// Still record the open event, but don't update the result status.
		_, err := r.createEvent(EventOpened, details)
		return err
	}
	return r.updateResultAtomic(EventOpened, details, func(e *Event) error {
		r.Status = EventOpened
		r.ModifiedDate = e.Time
		return nil
	})
}

// HandleClickedLink updates a Result in the case where the recipient clicked
// the link in an email.
func (r *Result) HandleClickedLink(details EventDetails) error {
	// Don't update the status if the user has already submitted data via the
	// landing page form.
	if r.Status == EventDataSubmit {
		_, err := r.createEvent(EventClicked, details)
		return err
	}
	return r.updateResultAtomic(EventClicked, details, func(e *Event) error {
		r.Status = EventClicked
		r.ModifiedDate = e.Time
		return nil
	})
}

// HandleFormSubmit updates a Result in the case where the recipient submitted
// credentials to the form on a Landing Page.
func (r *Result) HandleFormSubmit(details EventDetails) error {
	return r.updateResultAtomic(EventDataSubmit, details, func(e *Event) error {
		r.Status = EventDataSubmit
		r.ModifiedDate = e.Time
		return nil
	})
}

// HandleAttachmentOpen records an attachment-open event, distinguishing it
// from a regular link click (6.10). Status escalation mirrors HandleClickedLink.
func (r *Result) HandleAttachmentOpen(details EventDetails) error {
	if r.Status == EventDataSubmit {
		_, err := r.createEvent(EventAttachmentOpen, details)
		return err
	}
	return r.updateResultAtomic(EventAttachmentOpen, details, func(e *Event) error {
		if r.Status != EventClicked {
			r.Status = EventClicked
		}
		r.ModifiedDate = e.Time
		return nil
	})
}

// HandleCustomEvent updates a Result with a custom event (e.g. Word document opened, secondary link clicked).
// Requires a "title" field in the event payload.
func (r *Result) HandleCustomEvent(details EventDetails) error {
	eventTitle := details.Payload.Get("title")
	if eventTitle == "" {
		return errors.New("no title supplied for custom event")
	}
	event, err := r.createEvent(EventCustomEvent, details)
	if err != nil {
		return err
	}
	r.Status = eventTitle
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleSMSSent updates a Result to indicate that the SMS was successfully sent.
func (r *Result) HandleSMSSent() error {
	event, err := r.createEvent(EventSMSSent, nil)
	if err != nil {
		return err
	}
	r.SendDate = event.Time
	r.Status = EventSMSSent
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleSMSError updates a Result to indicate that there was an error sending the SMS.
func (r *Result) HandleSMSError(err error) error {
	event, err := r.createEvent(EventSMSError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = Error
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleSMSBackoff updates a Result to indicate the SMS needs a retry.
func (r *Result) HandleSMSBackoff(err error, sendDate time.Time) error {
	event, err := r.createEvent(EventSMSError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = StatusRetry
	r.SendDate = sendDate
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleMFACodeSent records when an MFA code is sent to the target.
func (r *Result) HandleMFACodeSent(details EventDetails) error {
	event, err := r.createEvent(EventMFACodeSent, details)
	if err != nil {
		return err
	}
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleMFACodeSendError records when an MFA SMS failed to be delivered.
func (r *Result) HandleMFACodeSendError(err error) error {
	event, createErr := r.createEvent(EventMFACodeSendError, EventError{Error: err.Error()})
	if createErr != nil {
		return createErr
	}
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleMFACodeVerified records when an MFA code is successfully verified.
func (r *Result) HandleMFACodeVerified(details EventDetails) error {
	event, err := r.createEvent(EventMFACodeVerified, details)
	if err != nil {
		return err
	}
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleMFACodeFailed records when an MFA code verification fails.
func (r *Result) HandleMFACodeFailed(details EventDetails) error {
	event, err := r.createEvent(EventMFACodeFailed, details)
	if err != nil {
		return err
	}
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleActivityInformation records that genuine human activity was detected
// (mouse movement, click, keypress). Fires on PATCH to phish handler.
// Does not overwrite EventDataSubmit — preserves highest-value status.
func (r *Result) HandleActivityInformation(details EventDetails) error {
	if r.Status == EventDataSubmit {
		return nil
	}
	event, err := r.createEvent(ActivityDetected, details)
	if err != nil {
		return err
	}
	r.Status = ActivityDetected
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailReport updates a Result in the case where they report a simulated
// phishing email using the HTTP handler.
func (r *Result) HandleEmailReport(details EventDetails) error {
	event, err := r.createEvent(EventReported, details)
	if err != nil {
		return err
	}
	r.Reported = true
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// UpdateGeo updates the latitude and longitude of the result in
// the database given an IP address
func (r *Result) UpdateGeo(addr string) error {
	// Open a connection to the maxmind db
	mmdb, err := maxminddb.Open("static/db/geolite2-city.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer mmdb.Close()
	ip := net.ParseIP(addr)
	var city mmCity
	// Get the record
	err = mmdb.Lookup(ip, &city)
	if err != nil {
		return err
	}
	// Update the database with the record information
	r.IP = addr
	r.Latitude = city.GeoPoint.Latitude
	r.Longitude = city.GeoPoint.Longitude
	return db.Save(r).Error
}

// defaultRIdCharset is the character set used for result ID generation
// when no campaign-specific override is configured.
const defaultRIdCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// defaultRIdLength is the default length of generated result IDs.
const defaultRIdLength = 7

func generateResultId() (string, error) {
	return generateResultIdCustom(defaultRIdCharset, defaultRIdLength)
}

// generateResultIdCustom generates a random ID using the given charset and
// length. Falls back to defaults when charset is empty or length is <= 0 (4.7).
func generateResultIdCustom(charset string, length int) (string, error) {
	if charset == "" {
		charset = defaultRIdCharset
	}
	if length <= 0 {
		length = defaultRIdLength
	}
	k := make([]byte, length)
	for i := range k {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		k[i] = charset[idx.Int64()]
	}
	return string(k), nil
}

// GenerateId generates a unique key to represent the result in the database.
// Optional charset and length parameters override the defaults (4.7).
func (r *Result) GenerateId(tx *gorm.DB, opts ...string) error {
	charset := ""
	length := 0
	if len(opts) >= 1 {
		charset = opts[0]
	}
	if len(opts) >= 2 {
		if n, err := strconv.Atoi(opts[1]); err == nil {
			length = n
		}
	}
	// Keep trying until we generate a unique key (shouldn't take more than one or two iterations)
	for {
		rid, err := generateResultIdCustom(charset, length)
		if err != nil {
			return err
		}
		r.RId = rid
		err = tx.Table("results").Where("r_id=?", r.RId).First(&Result{}).Error
		if err == gorm.ErrRecordNotFound {
			break
		}
	}
	return nil
}

// GetResult returns the Result object from the database
// given the ResultId
func GetResult(rid string) (Result, error) {
	r := Result{}
	err := db.Where("r_id=?", rid).First(&r).Error
	return r, err
}

// ResendResultByRId finds a specific result by its public RId and requeues it for sending.
func ResendResultByRId(rid string, user_id int64) error {
	r, err := GetResult(rid)
	if err != nil {
		return errors.New("Result not found")
	}

	// Verify the user has access to this campaign
	_, err = GetCampaign(r.CampaignId, user_id)
	if err != nil {
		return errors.New("access denied")
	}

	// Create a new MailLog entry to trigger the send operation by the mailer.
	m := &MailLog{
		CampaignId: r.CampaignId,
		UserId:     r.UserId,
		SendDate:   time.Now().UTC(),
		RId:        r.RId,
	}
	return db.Create(m).Error
}

// ResendAllResults finds all results for a given campaign and requeues them.
func ResendAllResults(campaign_id int64) error {
	results := []Result{}
	err := db.Where("campaign_id = ?", campaign_id).Find(&results).Error
	if err != nil {
		return err
	}
	for _, r := range results {
		m := &MailLog{
			CampaignId: r.CampaignId,
			UserId:     r.UserId,
			SendDate:   time.Now().UTC(),
			RId:        r.RId,
		}
		err = db.Create(m).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// MarkResultReported manually marks a result as reported and records a
// timeline event with the provided timestamp (6.6).
// If reportedAt is zero, the current UTC time is used.
func MarkResultReported(rid string, uid int64, reportedAt time.Time) error {
	r, err := GetResult(rid)
	if err != nil {
		return err
	}
	// Verify caller owns this campaign
	if _, err = GetCampaign(r.CampaignId, uid); err != nil {
		return errors.New("campaign not found or access denied")
	}
	if reportedAt.IsZero() {
		reportedAt = time.Now().UTC()
	}
	r.Reported = true
	r.ModifiedDate = reportedAt
	if err := db.Save(&r).Error; err != nil {
		return err
	}
	e := &Event{
		CampaignId: r.CampaignId,
		Email:      r.Email,
		Time:       reportedAt,
		Message:    EventReported,
	}
	return db.Save(e).Error
}

// DeleteResultCredentials removes captured credential data (event Details)
// from all Submitted Data events for the given RId while preserving the
// event metadata (timestamp, status, etc.) (6.5).
func DeleteResultCredentials(rid string, uid int64) error {
	r, err := GetResult(rid)
	if err != nil {
		return err
	}
	if _, err = GetCampaign(r.CampaignId, uid); err != nil {
		return errors.New("campaign not found or access denied")
	}
	return db.Model(&Event{}).
		Where("r_id=? AND message=?", rid, EventDataSubmit).
		UpdateColumn("details", "").Error
}

// ExcludeResult marks a result as excluded from the campaign, cancels any
// pending mail logs for it, and removes it from visible metrics (4.4).
// The result row itself is preserved for audit purposes.
func ExcludeResult(rid string, uid int64) error {
	r, err := GetResult(rid)
	if err != nil {
		return err
	}
	if _, err = GetCampaign(r.CampaignId, uid); err != nil {
		return errors.New("campaign not found or access denied")
	}
	// Delete any pending mail logs so no further emails are sent
	if err := db.Where("r_id=?", rid).Delete(&MailLog{}).Error; err != nil {
		log.Error(err)
	}
	// Persist the excluded status
	r.Status = "Excluded"
	r.ModifiedDate = time.Now().UTC()
	return db.Save(&r).Error
}

// CountMailLogs returns the number of MailLogs.
// This is a helper function intended for use in tests.
func CountMailLogs(cid int64) (int64, error) {
	var count int64
	err := db.Model(&MailLog{}).Where("campaign_id = ?", cid).Count(&count).Error
	return count, err
}

// GetFirstResultForCampaign returns the first result for a given campaign.
// This is a helper function intended for use in tests.
func GetFirstResultForCampaign(cid int64) (Result, error) {
	r := Result{}
	err := db.Where("campaign_id = ?", cid).First(&r).Error
	return r, err
}
