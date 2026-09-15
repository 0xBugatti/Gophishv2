package models

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net/url"
	"strconv"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/webhook"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

// Campaign is a struct representing a created campaign
type Campaign struct {
	Id            int64       `json:"id"`
	UserId        int64       `json:"-"`
	Name          string      `json:"name" sql:"not null"`
	CreatedDate   time.Time   `json:"created_date"`
	LaunchDate    time.Time   `json:"launch_date"`
	SendByDate    time.Time   `json:"send_by_date"`
	CompletedDate time.Time   `json:"completed_date"`
	TemplateId    int64       `json:"-"`
	Template      Template    `json:"template"`
	PageId        int64       `json:"-"`
	Page          Page        `json:"page"`
	Status        string      `json:"status"`
	Results       []Result    `json:"results,omitempty"`
	Groups        []Group     `json:"groups,omitempty"`
	Events        []Event     `json:"timeline,omitempty"`
	SMTPId        int64       `json:"-"`
	SMTP          SMTP        `json:"smtp"`
	URL           string      `json:"url"`
	EncryptionKey string      `json:"encryption_key"`
	StartTime     time.Time   `json:"start_time"`
	EndTime       time.Time   `json:"end_time"`
	Location      string      `json:"location"`
	Type          string      `json:"type" sql:"default:'email'"`
	SMSTemplateId int64       `json:"-"`
	SMSTemplate   SMSTemplate `json:"sms_template,omitempty"`
	SMSId         int64       `json:"-"`
	SMS           SMS         `json:"sms,omitempty"`
	URLParam      string      `json:"urlparam" sql:"column:urlparam"`
	QRSize        string      `json:"qrsize" sql:"column:qrsize"`
	HTTPAuth      bool        `json:"basicauth" sql:"column:basicauth"`
	CampaignSetId int64       `json:"campaign_set_id,omitempty"`
	// SendIntervalMs is the fixed delay between individual email dispatches (ms).
	// When non-zero, takes precedence over the SMTP profile's send_rate limiter.
	SendIntervalMs int `json:"send_interval_ms" gorm:"column:send_interval_ms"`
	// RandomizeSendOrder shuffles the target list before dispatch using
	// a crypto/rand-seeded Fisher-Yates algorithm to make sends less predictable.
	RandomizeSendOrder bool `json:"randomize_send_order" gorm:"column:randomize_send_order"`
	// Description is an optional human-readable note visible only in the UI.
	Description string `json:"description" gorm:"column:description"`
	// SendConcurrency controls how many parallel goroutines dispatch emails
	// within a single campaign launch. 0 or 1 = serial (default) (2.4).
	SendConcurrency int `json:"send_concurrency" gorm:"column:send_concurrency"`
	// RIdCharset overrides the character set used for result ID generation (4.7).
	// Empty = default alphanumeric. Example: "abcdef0123456789" for hex-only.
	RIdCharset string `json:"rid_charset" gorm:"column:rid_charset"`
	// RIdLength overrides the length of generated result IDs (4.7).
	// 0 = default (7 characters).
	RIdLength int `json:"rid_length" gorm:"column:rid_length"`
	// TemplateVariants stores JSON array of weighted template variants for
	// A/B testing (3.8). Format: [{"template_id":1,"weight":50},{"template_id":2,"weight":50}].
	// When empty, the campaign's primary template is used for all targets.
	TemplateVariants string `json:"template_variants" gorm:"column:template_variants;type:text"`
}

// TemplateVariant represents one weighted template in an A/B test (3.8).
type TemplateVariant struct {
	TemplateId int64 `json:"template_id"`
	Weight     int   `json:"weight"`
}

// PickTemplateVariant parses TemplateVariants JSON and returns a weighted-
// random template ID. If no variants are configured, returns 0 (use default).
func (c *Campaign) PickTemplateVariant() int64 {
	if c.TemplateVariants == "" {
		return 0
	}
	var variants []TemplateVariant
	if err := json.Unmarshal([]byte(c.TemplateVariants), &variants); err != nil || len(variants) == 0 {
		return 0
	}
	totalWeight := 0
	for _, v := range variants {
		totalWeight += v.Weight
	}
	if totalWeight <= 0 {
		return variants[0].TemplateId
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(totalWeight)))
	pick := int(n.Int64())
	cumulative := 0
	for _, v := range variants {
		cumulative += v.Weight
		if pick < cumulative {
			return v.TemplateId
		}
	}
	return variants[len(variants)-1].TemplateId
}

// CampaignResults is a struct representing the results from a campaign
type CampaignResults struct {
	Id      int64    `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Results []Result `json:"results,omitempty"`
	Events  []Event  `json:"timeline,omitempty"`
}

// CampaignSummaries is a struct representing the overview of campaigns
type CampaignSummaries struct {
	Total     int64             `json:"total"`
	Campaigns []CampaignSummary `json:"campaigns"`
}

// CampaignSummary is a struct representing the overview of a single camaign
type CampaignSummary struct {
	Id            int64         `json:"id"`
	CreatedDate   time.Time     `json:"created_date"`
	LaunchDate    time.Time     `json:"launch_date"`
	SendByDate    time.Time     `json:"send_by_date"`
	CompletedDate time.Time     `json:"completed_date"`
	Status        string        `json:"status"`
	Name          string        `json:"name"`
	Stats         CampaignStats `json:"stats"`
}

// CampaignStats is a struct representing the statistics for a single campaign
type CampaignStats struct {
	Total         int64 `json:"total"`
	EmailsSent    int64 `json:"sent"`
	OpenedEmail   int64 `json:"opened"`
	ClickedLink   int64 `json:"clicked"`
	SubmittedData int64 `json:"submitted_data"`
	EmailReported int64 `json:"email_reported"`
	Error         int64 `json:"error"`
}

// Event contains the fields for an event
// that occurs during the campaign
type Event struct {
	Id            int64     `json:"id"`
	CampaignId    int64     `json:"campaign_id"`
	Email         string    `json:"email"`
	Time          time.Time `json:"time"`
	Message       string    `json:"message"`
	Details       string    `json:"details"`
	FalsePositive bool      `json:"false_positive"`
}

// EventDetails is a struct that wraps common attributes we want to store
// in an event
type EventDetails struct {
	Payload url.Values        `json:"payload"`
	Browser map[string]string `json:"browser"`
}

// EventError is a struct that wraps an error that occurs when sending an
// email to a recipient
type EventError struct {
	Error string `json:"error"`
}

// ErrCampaignNameNotSpecified indicates there was no template given by the user
var ErrCampaignNameNotSpecified = errors.New("Campaign name not specified")

// ErrGroupNotSpecified indicates there was no template given by the user
var ErrGroupNotSpecified = errors.New("No groups specified")

// ErrTemplateNotSpecified indicates there was no template given by the user
var ErrTemplateNotSpecified = errors.New("No email template specified")

// ErrPageNotSpecified indicates a landing page was not provided for the campaign
var ErrPageNotSpecified = errors.New("No landing page specified")

// ErrSMTPNotSpecified indicates a sending profile was not provided for the campaign
var ErrSMTPNotSpecified = errors.New("No sending profile specified")

// ErrTemplateNotFound indicates the template specified does not exist in the database
var ErrTemplateNotFound = errors.New("Template not found")

// ErrGroupNotFound indicates a group specified by the user does not exist in the database
var ErrGroupNotFound = errors.New("Group not found")

// ErrPageNotFound indicates a page specified by the user does not exist in the database
var ErrPageNotFound = errors.New("Page not found")

// ErrSMTPNotFound indicates a sending profile specified by the user does not exist in the database
var ErrSMTPNotFound = errors.New("Sending profile not found")

// ErrInvalidSendByDate indicates that the user specified a send by date that occurs before the
// launch date
var ErrInvalidSendByDate = errors.New("The launch date must be before the \"send emails by\" date")

// ErrSMSNotSpecified indicates an SMS sending profile was not provided for the campaign
var ErrSMSNotSpecified = errors.New("No SMS sending profile specified")

// ErrSMSTemplateNotSpecified indicates there was no SMS template given by the user
var ErrSMSTemplateNotSpecified = errors.New("No SMS template specified")

// ErrSMSTemplateNotFound indicates the SMS template specified does not exist in the database
var ErrSMSTemplateNotFound = errors.New("SMS template not found")

// ErrSMSNotFound indicates an SMS sending profile specified by the user does not exist in the database
var ErrSMSNotFound = errors.New("SMS sending profile not found")

// ErrInvalidCampaignID is thrown when a campaign ID is invalid
var ErrInvalidCampaignID = errors.New("Invalid campaign ID")

// RecipientParameter is the URL parameter that points to the result ID for a recipient.
const RecipientParameter = "userid"

// Validate checks to make sure there are no invalid fields in a submitted campaign
func (c *Campaign) Validate() error {
	switch {
	case c.Name == "":
		return ErrCampaignNameNotSpecified
	case len(c.Groups) == 0:
		return ErrGroupNotSpecified
	case c.Template.Name == "":
		return ErrTemplateNotSpecified
	case c.Page.Name == "":
		return ErrPageNotSpecified
	case c.SMTP.Name == "":
		return ErrSMTPNotSpecified
	case !c.SendByDate.IsZero() && !c.LaunchDate.IsZero() && c.SendByDate.Before(c.LaunchDate):
		return ErrInvalidSendByDate
	}
	return nil
}

// UpdateStatus changes the campaign status appropriately
func (c *Campaign) UpdateStatus(s string) error {
	// This could be made simpler, but I think there's a bug in gorm
	return db.Table("campaigns").Where("id=?", c.Id).Update("status", s).Error
}

// Sets the False Positive-field for the specified eventid to true.
// Checks if all Events with the specific email in that campaign where data has been submitted are set to false_positive "true" and if thats the case changes
// the status in the "results" table for that recipient back to "Clicked Link" hence it won´t be count in the Submitted statistic until new data is submitted
func MarkEvent(eveId int64, rid string) error {
	var mailcount int64
	var fpcount int64
	s := Event{}
	query := db.Table("events").Where("id=?", eveId).Update("false_positive", true).Error
	if query != nil {
		log.Errorf("Problem editing false_positive in database: Table \"events\" on id %d", eveId)
	}
	mailquery := db.Table("events").Where("id = ?", eveId)
	mailquery.Select("id, campaign_id, email, time, message, details, false_positive")
	err := mailquery.Scan(&s).Error
	if err != nil {
		log.Error(err)
		return err
	}
	//Counter to check if all Data Submitted is flaged as false/positive
	db.Table("events").Select("Count(*)").Where("email = ? AND message = ?", s.Email, "Submitted Data").Count(&mailcount)
	db.Table("events").Select("Count(*)").Where("email = ? AND message = ? AND false_positive = ?", s.Email, "Submitted Data", true).Count(&fpcount)
	if mailcount == fpcount {
		return db.Table("results").Where("campaign_id = ? AND r_id = ?", s.CampaignId, rid).Update("status", "Clicked Link").Error
	}
	return query
}

// WebhookPayload wraps an Event with campaign metadata for enriched webhook
// payloads (6.9). External integrations receive subject, template name, and
// SMTP profile alongside every event.
type WebhookPayload struct {
	*Event
	CampaignName string `json:"campaign_name,omitempty"`
	TemplateName string `json:"template_name,omitempty"`
	SMTPName     string `json:"smtp_name,omitempty"`
	Subject      string `json:"subject,omitempty"`
}

// enrichEventForWebhook wraps an event with campaign metadata (6.9).
func enrichEventForWebhook(e *Event) interface{} {
	c := Campaign{}
	err := db.Where("id = ?", e.CampaignId).First(&c).Error
	if err != nil {
		return e // fall back to bare event
	}
	t := Template{}
	_ = db.Where("id = ?", c.TemplateId).First(&t).Error
	s := SMTP{}
	_ = db.Where("id = ?", c.SMTPId).First(&s).Error
	return &WebhookPayload{
		Event:        e,
		CampaignName: c.Name,
		TemplateName: t.Name,
		SMTPName:     s.Name,
		Subject:      t.Subject,
	}
}

// AddEvent creates a new campaign event in the database
// EventBroadcastFunc is a callback invoked after every event is persisted.
// Controllers wire this to push events to SSE clients (7.4). Setting it
// avoids an import cycle between models and controllers/api.
var EventBroadcastFunc func(uid int64, event interface{})

func AddEvent(e *Event, campaignID int64) error {
	e.CampaignId = campaignID
	e.Time = time.Now().UTC()

	whs, err := GetActiveWebhooks()
	if err == nil {
		whEndPoints := []webhook.EndPoint{}
		for _, wh := range whs {
			whEndPoints = append(whEndPoints, webhook.EndPoint{
				URL:    wh.URL,
				Secret: wh.Secret,
			})
		}
		webhook.SendAll(whEndPoints, enrichEventForWebhook(e))
	} else {
		log.Errorf("error getting active webhooks: %v", err)
	}

	if saveErr := db.Save(e).Error; saveErr != nil {
		return saveErr
	}

	// Broadcast to SSE listeners (7.4).
	if EventBroadcastFunc != nil {
		// Look up the campaign owner for scoping.
		c := Campaign{}
		if db.Select("user_id").Where("id = ?", campaignID).First(&c).Error == nil {
			go EventBroadcastFunc(c.UserId, e)
		}
	}

	return nil
}

// getDetails retrieves the related attributes of the campaign
// from the database. If the Events and the Results are not available,
// an error is returned. Otherwise, the attribute name is set to [Deleted],
// indicating the user deleted the attribute (template, smtp, etc.)
func (c *Campaign) getDetails() error {
	err := db.Model(c).Related(&c.Results).Error
	if err != nil {
		log.Warnf("%s: results not found for campaign", err)
		return err
	}
	err = db.Model(c).Related(&c.Events).Error
	if err != nil {
		log.Warnf("%s: events not found for campaign", err)
		return err
	}
	err = db.Table("templates").Where("id=?", c.TemplateId).First(&c.Template).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.Template = Template{Name: "[Deleted]"}
		log.Warnf("%s: template not found for campaign", err)
	}
	err = db.Where("template_id=?", c.Template.Id).Find(&c.Template.Attachments).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Warn(err)
		return err
	}
	err = db.Table("pages").Where("id=?", c.PageId).First(&c.Page).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.Page = Page{Name: "[Deleted]"}
		log.Warnf("%s: page not found for campaign", err)
	}
	err = db.Table("smtp").Where("id=?", c.SMTPId).First(&c.SMTP).Error
	if err != nil {
		// Check if the SMTP was deleted
		if err != gorm.ErrRecordNotFound {
			return err
		}
		c.SMTP = SMTP{Name: "[Deleted]"}
		log.Warnf("%s: sending profile not found for campaign", err)
	}
	err = db.Where("smtp_id=?", c.SMTP.Id).Find(&c.SMTP.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Warn(err)
		return err
	}
	return nil
}

// getBaseURL returns the Campaign's configured URL.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getBaseURL() string {
	return c.URL
}

// getFromAddress returns the Campaign's configured SMTP "From" address.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getFromAddress() string {
	return c.SMTP.FromAddress
}

// getEncryptionKey returns the Campaign's lure URL encryption key.
// This is used to implement the TemplateContext interface.
func (c *Campaign) getEncryptionKey() string {
	return c.EncryptionKey
}

// generateSendDate creates a sendDate
func (c *Campaign) generateSendDate(idx int, totalRecipients int) time.Time {
	// If no send date is specified, just return the launch date
	if c.SendByDate.IsZero() || c.SendByDate.Equal(c.LaunchDate) {
		return c.LaunchDate
	}
	// Otherwise, we can calculate the range of minutes to send emails
	// (since we only poll once per minute)
	totalMinutes := c.SendByDate.Sub(c.LaunchDate).Minutes()

	// Next, we can determine how many minutes should elapse between emails
	minutesPerEmail := totalMinutes / float64(totalRecipients)

	// Then, we can calculate the offset for this particular email
	offset := int(minutesPerEmail * float64(idx))

	// Finally, we can just add this offset to the launch date to determine
	// when the email should be sent
	return c.LaunchDate.Add(time.Duration(offset) * time.Minute)
}

// getCampaignStats returns a CampaignStats object for the campaign with the given campaign ID.
// It also backfills numbers as appropriate with a running total, so that the values are aggregated.
func getCampaignStats(cid int64) (CampaignStats, error) {
	s := CampaignStats{}
	query := db.Table("results").Where("campaign_id = ?", cid)
	err := query.Count(&s.Total).Error
	if err != nil {
		return s, err
	}
	query.Where("status=?", EventDataSubmit).Count(&s.SubmittedData)
	if err != nil {
		return s, err
	}
	query.Where("status=?", EventClicked).Count(&s.ClickedLink)
	if err != nil {
		return s, err
	}
	query.Where("reported=?", true).Count(&s.EmailReported)
	if err != nil {
		return s, err
	}
	// Every submitted data event implies they clicked the link
	s.ClickedLink += s.SubmittedData
	err = query.Where("status=?", EventOpened).Count(&s.OpenedEmail).Error
	if err != nil {
		return s, err
	}
	// Every clicked link event implies they opened the email
	s.OpenedEmail += s.ClickedLink
	err = query.Where("status=?", EventSent).Count(&s.EmailsSent).Error
	if err != nil {
		return s, err
	}
	// Every opened email event implies the email was sent
	s.EmailsSent += s.OpenedEmail
	err = query.Where("status=?", Error).Count(&s.Error).Error
	return s, err
}

// GetCampaigns returns the campaigns owned by the given user.
func GetCampaigns(uid int64) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Model(&User{Id: uid}).Related(&cs).Error
	if err != nil {
		log.Error(err)
	}
	for i := range cs {
		err = cs[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}
	return cs, err
}

// GetCampaignSummaries gets the summary objects for all the campaigns
// owned by the current user
func GetCampaignSummaries(uid int64) (CampaignSummaries, error) {
	overview := CampaignSummaries{}
	cs := []CampaignSummary{}
	// Get the basic campaign information
	query := db.Table("campaigns").Where("user_id = ?", uid)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status")
	err := query.Scan(&cs).Error
	if err != nil {
		log.Error(err)
		return overview, err
	}
	for i := range cs {
		s, err := getCampaignStats(cs[i].Id)
		if err != nil {
			log.Error(err)
			return overview, err
		}
		cs[i].Stats = s
	}
	overview.Total = int64(len(cs))
	overview.Campaigns = cs
	return overview, nil
}

// GetCampaignSummary gets the summary object for a campaign specified by the campaign ID
func GetCampaignSummary(id int64, uid int64) (CampaignSummary, error) {
	cs := CampaignSummary{}
	query := db.Table("campaigns").Where("user_id = ? AND id = ?", uid, id)
	query = query.Select("id, name, created_date, launch_date, send_by_date, completed_date, status")
	err := query.Scan(&cs).Error
	if err != nil {
		log.Error(err)
		return cs, err
	}
	s, err := getCampaignStats(cs.Id)
	if err != nil {
		log.Error(err)
		return cs, err
	}
	cs.Stats = s
	return cs, nil
}

// GetCampaignMailContext returns a campaign object with just the relevant
// data needed to generate and send emails. This includes the top-level
// metadata, the template, and the sending profile.
//
// This should only ever be used if you specifically want this lightweight
// context, since it returns a non-standard campaign object.
// ref: #1726
func GetCampaignMailContext(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).First(&c).Error
	if err != nil {
		return c, err
	}
	err = db.Table("smtp").Where("id=?", c.SMTPId).First(&c.SMTP).Error
	if err != nil {
		return c, err
	}
	err = db.Where("smtp_id=?", c.SMTP.Id).Find(&c.SMTP.Headers).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return c, err
	}
	err = db.Table("templates").Where("id=?", c.TemplateId).First(&c.Template).Error
	if err != nil {
		return c, err
	}
	err = db.Where("template_id=?", c.Template.Id).Find(&c.Template.Attachments).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return c, err
	}
	return c, nil
}

// GetCampaign returns the campaign, if it exists, specified by the given id and user_id.
func GetCampaign(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).First(&c).Error
	if err != nil {
		log.Errorf("%s: campaign not found", err)
		return c, err
	}
	err = c.getDetails()
	return c, err
}

// GetCampaignResults returns just the campaign results for the given campaign
func GetCampaignResults(id int64, uid int64) (CampaignResults, error) {
	cr := CampaignResults{}
	err := db.Table("campaigns").Where("id=? and user_id=?", id, uid).Find(&cr).Error
	if err != nil {
		log.WithFields(logrus.Fields{
			"campaign_id": id,
			"error":       err,
		}).Error(err)
		return cr, err
	}
	err = db.Table("results").Where("campaign_id=? and user_id=?", cr.Id, uid).Find(&cr.Results).Error
	if err != nil {
		log.Errorf("%s: results not found for campaign", err)
		return cr, err
	}
	err = db.Table("events").Where("campaign_id=?", cr.Id).Find(&cr.Events).Error
	if err != nil {
		log.Errorf("%s: events not found for campaign", err)
		return cr, err
	}
	return cr, err
}

// GetCampaignIdsByGroupId returns campaign IDs that contain targets whose
// email addresses match members of the given group. Since GoPhish denormalizes
// group targets into results, this performs a subquery join. Scoped to uid (6.2).
func GetCampaignIdsByGroupId(gid, uid int64) ([]int64, error) {
	// Fetch group target emails.
	g, err := GetGroup(gid, uid)
	if err != nil {
		return nil, err
	}
	emails := make([]string, len(g.Targets))
	for i, t := range g.Targets {
		emails[i] = t.Email
	}
	if len(emails) == 0 {
		return nil, nil
	}
	type row struct{ CampaignId int64 }
	var rows []row
	err = db.Table("results").
		Select("DISTINCT campaign_id").
		Joins("JOIN campaigns ON campaigns.id = results.campaign_id").
		Where("results.email IN (?) AND campaigns.user_id = ?", emails, uid).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.CampaignId
	}
	return ids, nil
}

// GetExpiredCampaigns returns in-progress campaigns whose EndTime has passed (4.5).
// These should be automatically transitioned to Completed by the worker.
func GetExpiredCampaigns(t time.Time) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Where("end_time != ? AND end_time <= ?", time.Time{}, t).
		Where("status IN (?)", []string{CampaignInProgress, CampaignPaused}).
		Find(&cs).Error
	if err != nil {
		log.Error(err)
	}
	return cs, err
}

// GetQueuedCampaigns returns the campaigns that are queued up for this given minute
func GetQueuedCampaigns(t time.Time) ([]Campaign, error) {
	cs := []Campaign{}
	err := db.Where("launch_date <= ?", t).
		Where("status = ?", CampaignQueued).Find(&cs).Error
	if err != nil {
		log.Error(err)
	}
	log.Infof("Found %d Campaigns to run\n", len(cs))
	for i := range cs {
		err = cs[i].getDetails()
		if err != nil {
			log.Error(err)
		}
	}
	return cs, err
}

// PostCampaign inserts a campaign and all associated records into the database.
func PostCampaign(c *Campaign, uid int64) error {
	err := c.Validate()
	if err != nil {
		return err
	}
	// Fill in the details
	c.UserId = uid
	c.CreatedDate = time.Now().UTC()
	c.CompletedDate = time.Time{}
	c.Status = CampaignQueued
	if c.LaunchDate.IsZero() {
		c.LaunchDate = c.CreatedDate
	} else {
		c.LaunchDate = c.LaunchDate.UTC()
	}
	if !c.SendByDate.IsZero() {
		c.SendByDate = c.SendByDate.UTC()
	}
	if c.LaunchDate.Before(c.CreatedDate) || c.LaunchDate.Equal(c.CreatedDate) {
		c.Status = CampaignInProgress
	}
	// Check to make sure all the groups already exist
	// Also, later we'll need to know the total number of recipients (counting
	// duplicates is ok for now), so we'll do that here to save a loop.
	totalRecipients := 0
	for i, g := range c.Groups {
		c.Groups[i], err = GetGroupByName(g.Name, uid)
		if err == gorm.ErrRecordNotFound {
			log.WithFields(logrus.Fields{
				"group": g.Name,
			}).Error("Group does not exist")
			return ErrGroupNotFound
		} else if err != nil {
			log.Error(err)
			return err
		}
		totalRecipients += len(c.Groups[i].Targets)
	}
	// Check to make sure the template exists
	t, err := GetTemplateByName(c.Template.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"template": c.Template.Name,
		}).Error("Template does not exist")
		return ErrTemplateNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.Template = t
	c.TemplateId = t.Id
	// Check to make sure the page exists
	p, err := GetPageByName(c.Page.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"page": c.Page.Name,
		}).Error("Page does not exist")
		return ErrPageNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.Page = p
	c.PageId = p.Id
	// Check to make sure the sending profile exists
	s, err := GetSMTPByName(c.SMTP.Name, uid)
	if err == gorm.ErrRecordNotFound {
		log.WithFields(logrus.Fields{
			"smtp": c.SMTP.Name,
		}).Error("Sending profile does not exist")
		return ErrSMTPNotFound
	} else if err != nil {
		log.Error(err)
		return err
	}
	c.SMTP = s
	c.SMTPId = s.Id
	// Insert into the DB
	err = db.Save(c).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = AddEvent(&Event{Message: "Campaign Created"}, c.Id)
	if err != nil {
		log.Error(err)
	}
	// Insert all the results
	resultMap := make(map[string]bool)
	recipientIndex := 0
	tx := db.Begin()
	for _, g := range c.Groups {
		// Insert a result for each target in the group
		for _, t := range g.Targets {
			// Remove duplicate results - we should only
			// send emails to unique email addresses.
			if _, ok := resultMap[t.Email]; ok {
				continue
			}
			resultMap[t.Email] = true
			sendDate := c.generateSendDate(recipientIndex, totalRecipients)
			r := &Result{
				BaseRecipient: BaseRecipient{
					Email:     t.Email,
					Position:  t.Position,
					FirstName: t.FirstName,
					LastName:  t.LastName,
				},
				Status:       StatusScheduled,
				CampaignId:   c.Id,
				UserId:       c.UserId,
				SendDate:     sendDate,
				Reported:     false,
				ModifiedDate: c.CreatedDate,
			}
			err = r.GenerateId(tx, c.RIdCharset, strconv.Itoa(c.RIdLength))
			if err != nil {
				log.Error(err)
				tx.Rollback()
				return err
			}
			processing := false
			if r.SendDate.Before(c.CreatedDate) || r.SendDate.Equal(c.CreatedDate) {
				r.Status = StatusSending
				processing = true
			}
			err = tx.Save(r).Error
			if err != nil {
				log.WithFields(logrus.Fields{
					"email": t.Email,
				}).Errorf("error creating result: %v", err)
				tx.Rollback()
				return err
			}
			c.Results = append(c.Results, *r)
			log.WithFields(logrus.Fields{
				"email":     r.Email,
				"send_date": sendDate,
			}).Debug("creating maillog")
			m := &MailLog{
				UserId:     c.UserId,
				CampaignId: c.Id,
				RId:        r.RId,
				SendDate:   sendDate,
				Processing: processing,
			}
			err = tx.Save(m).Error
			if err != nil {
				log.WithFields(logrus.Fields{
					"email": t.Email,
				}).Errorf("error creating maillog entry: %v", err)
				tx.Rollback()
				return err
			}
			recipientIndex++
		}
	}
	return tx.Commit().Error
}

// DeleteCampaign deletes the specified campaign
func DeleteCampaign(id int64) error {
	log.WithFields(logrus.Fields{
		"campaign_id": id,
	}).Info("Deleting campaign")
	// Delete all the campaign results
	err := db.Where("campaign_id=?", id).Delete(&Result{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&Event{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	err = db.Where("campaign_id=?", id).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Delete the campaign
	err = db.Delete(&Campaign{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// CompleteCampaign effectively "ends" a campaign.
// Any future emails clicked will return a simple "404" page.
func CompleteCampaign(id int64, uid int64) error {
	log.WithFields(logrus.Fields{
		"campaign_id": id,
	}).Info("Marking campaign as complete")
	c, err := GetCampaign(id, uid)
	if err != nil {
		return err
	}
	// Delete any maillogs still set to be sent out, preventing future emails
	err = db.Where("campaign_id=?", id).Delete(&MailLog{}).Error
	if err != nil {
		log.Error(err)
		return err
	}
	// Don't overwrite original completed time
	if c.Status == CampaignComplete {
		return nil
	}
	// Mark the campaign as complete
	c.CompletedDate = time.Now().UTC()
	c.Status = CampaignComplete
	err = db.Model(&Campaign{}).Where("id=? and user_id=?", id, uid).
		Select([]string{"completed_date", "status"}).UpdateColumns(&c).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// PauseCampaign sets a campaign's status to Paused so the worker skips its
// queued mail logs until the campaign is resumed (4.1).
func PauseCampaign(id int64, uid int64) error {
	c, err := GetCampaign(id, uid)
	if err != nil {
		return err
	}
	if c.Status == CampaignComplete || c.Status == CampaignPaused {
		return nil
	}
	return db.Model(&Campaign{}).Where("id=? and user_id=?", id, uid).
		UpdateColumn("status", CampaignPaused).Error
}

// ResumeCampaign moves a paused campaign back to In Progress so the worker
// resumes dispatching its queued mail logs (4.1).
func ResumeCampaign(id int64, uid int64) error {
	c, err := GetCampaign(id, uid)
	if err != nil {
		return err
	}
	if c.Status != CampaignPaused {
		return nil
	}
	return db.Model(&Campaign{}).Where("id=? and user_id=?", id, uid).
		UpdateColumn("status", CampaignInProgress).Error
}

// PutCampaign updates editable fields of an existing campaign (4.2).
// Only campaigns in Queued status allow full edits (name, URL, template, SMTP).
// In-progress or paused campaigns only allow name, description, end_time, and
// send_by_date to be modified.
func PutCampaign(c *Campaign, uid int64) error {
	existing, err := GetCampaign(c.Id, uid)
	if err != nil {
		return err
	}
	if existing.Status == CampaignComplete {
		return errors.New("Cannot edit a completed campaign")
	}
	switch existing.Status {
	case CampaignQueued:
		// Queued campaigns allow editing most fields
		existing.Name = c.Name
		existing.Description = c.Description
		existing.URL = c.URL
		if !c.SendByDate.IsZero() {
			existing.SendByDate = c.SendByDate.UTC()
		}
		if !c.EndTime.IsZero() {
			existing.EndTime = c.EndTime.UTC()
		}
		return db.Where("id=? AND user_id=?", c.Id, uid).Save(&existing).Error
	default:
		// In-progress / paused — only safe metadata
		updates := map[string]interface{}{
			"name":        c.Name,
			"description": c.Description,
		}
		if !c.SendByDate.IsZero() {
			updates["send_by_date"] = c.SendByDate.UTC()
		}
		if !c.EndTime.IsZero() {
			updates["end_time"] = c.EndTime.UTC()
		}
		return db.Model(&Campaign{}).Where("id=? AND user_id=?", c.Id, uid).UpdateColumns(updates).Error
	}
}

// GetCampaignSMSContext returns a campaign with just the SMS-relevant data
// needed to generate and send SMS messages.
func GetCampaignSMSContext(id int64, uid int64) (Campaign, error) {
	c := Campaign{}
	err := db.Where("id = ?", id).Where("user_id = ?", uid).First(&c).Error
	if err != nil {
		return c, err
	}
	if c.Type != "sms" {
		return c, errors.New("attempted to get SMS context for a non-SMS campaign")
	}
	err = db.Table("sms_profiles").Where("id=?", c.SMSId).First(&c.SMS).Error
	if err != nil {
		return c, err
	}
	err = db.Table("sms_templates").Where("id=?", c.SMSTemplateId).First(&c.SMSTemplate).Error
	if err != nil {
		return c, err
	}
	return c, nil
}

// CampaignLink is the response payload for the generate-link endpoint.
type CampaignLink struct {
	URL  string `json:"url"`
	RId  string `json:"rid"`
	Name string `json:"name"`
}

// GenerateCampaignLink creates a new Result row with a unique RId that can be
// embedded in documents or other non-email materials for visit tracking.
// The generated phishing URL follows the same ?userid=<rid> convention used
// by email campaigns so the phish server handles it identically.
func GenerateCampaignLink(cid, uid int64, name string) (CampaignLink, error) {
	c, err := GetCampaign(cid, uid)
	if err != nil {
		return CampaignLink{}, err
	}
	r := Result{
		CampaignId: c.Id,
		UserId:     uid,
		Status:     StatusScheduled,
		BaseRecipient: BaseRecipient{
			FirstName: name,
		},
	}
	tx := db.Begin()
	if err := r.GenerateId(tx, c.RIdCharset, strconv.Itoa(c.RIdLength)); err != nil {
		tx.Rollback()
		return CampaignLink{}, err
	}
	if err := tx.Save(&r).Error; err != nil {
		tx.Rollback()
		return CampaignLink{}, err
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return CampaignLink{}, err
	}
	u, _ := url.Parse(c.URL)
	q := u.Query()
	q.Set(RecipientParameter, r.RId)
	u.RawQuery = q.Encode()
	return CampaignLink{URL: u.String(), RId: r.RId, Name: name}, nil
}
