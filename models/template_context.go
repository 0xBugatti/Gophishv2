package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/gophish/gophish/evilginx"
)

// TemplateContext is an interface that allows both campaigns and email
// requests to have a PhishingTemplateContext generated for them.
type TemplateContext interface {
	getFromAddress() string
	getBaseURL() string
	getEncryptionKey() string
}

// PhishingTemplateContext is the context that is sent to any template, such
// as the email or landing page content.
type PhishingTemplateContext struct {
	From          string
	URL           string
	Tracker       string
	TrackingURL   string
	RId           string
	BaseURL       string
	EncryptionKey string
	// Extended variables (3.5)
	Domain    string // email domain of the target (part after @)
	FromEmail string // raw sender email address from the SMTP profile
	Date      string // formatted send date (YYYY-MM-DD)
	Time      string // formatted send time (HH:MM)
	// CustomFields exposes arbitrary per-target fields from CSV import as
	// {{.CustomFields.ColumnName}} in templates (5.7). Parsed from the
	// BaseRecipient.Custom JSON string.
	CustomFields map[string]string
	BaseRecipient
}

// NewPhishingTemplateContext returns a populated PhishingTemplateContext,
// parsing the correct fields from the provided TemplateContext and recipient.
func NewPhishingTemplateContext(ctx TemplateContext, r BaseRecipient, rid string) (PhishingTemplateContext, error) {
	f, err := mail.ParseAddress(ctx.getFromAddress())
	if err != nil {
		return PhishingTemplateContext{}, err
	}
	fn := f.Name
	if fn == "" {
		fn = f.Address
	}
	templateURL, err := ExecuteTemplate(ctx.getBaseURL(), r)
	if err != nil {
		return PhishingTemplateContext{}, err
	}

	// For the base URL, strip path and query → http://example.com
	baseURL, err := url.Parse(templateURL)
	if err != nil {
		return PhishingTemplateContext{}, err
	}
	baseURL.Path = ""
	baseURL.RawQuery = ""

	baseLureURL, err := url.Parse(templateURL)
	if err != nil {
		return PhishingTemplateContext{}, err
	}

	encKey := ctx.getEncryptionKey()

	// Extract any encrypted lure params from the URL before building phish link
	q := url.Values{}
	urlQuery := url.Values{}
	for k, v := range baseLureURL.Query() {
		params, ok, _ := evilginx.ExtractPhishUrlParams(v[0], encKey)
		if ok {
			for pk, pv := range params {
				q.Set(pk, pv)
			}
		} else {
			urlQuery.Set(k, v[0])
		}
	}
	baseLureURL.RawQuery = urlQuery.Encode()

	phishURL := *baseLureURL
	q.Set(RecipientParameter, rid)
	evilginx.AddPhishUrlParams(&phishURL, q, encKey)

	trackingURL := *baseLureURL
	tq := url.Values{}
	tq.Set(RecipientParameter, rid)
	tq.Set("o", "track")
	evilginx.AddPhishUrlParams(&trackingURL, tq, encKey)

	// Derive extended variables
	domain := ""
	if idx := strings.LastIndex(r.Email, "@"); idx >= 0 {
		domain = r.Email[idx+1:]
	}
	now := time.Now().UTC()

	// Parse custom fields from the JSON-encoded Custom string (5.7).
	customFields := make(map[string]string)
	if r.Custom != "" {
		_ = json.Unmarshal([]byte(r.Custom), &customFields)
	}

	return PhishingTemplateContext{
		BaseRecipient: r,
		BaseURL:       baseURL.String(),
		URL:           phishURL.String(),
		TrackingURL:   trackingURL.String(),
		Tracker:       "<img alt='' style='display: none' src='" + trackingURL.String() + "'/>",
		From:          fn,
		RId:           rid,
		EncryptionKey: encKey,
		Domain:        domain,
		FromEmail:     f.Address,
		Date:          now.Format("2006-01-02"),
		Time:          now.Format("15:04"),
		CustomFields:  customFields,
	}, nil
}

// ExecuteTemplate creates a templated string based on the provided
// template body and data.
// templateFuncs returns the custom FuncMap available in all templates.
// Currently provides {{include "TemplateName"}} for nested template
// inclusion (7.13). The included template is rendered with the same data
// context as the parent. Recursion is capped at 5 levels.
func templateFuncs(data interface{}, depth int) template.FuncMap {
	return template.FuncMap{
		"include": func(name string) (string, error) {
			if depth >= 5 {
				return "", fmt.Errorf("include: max nesting depth (5) exceeded")
			}
			// Resolve the template by name. We try all users since
			// includes happen at render time with the template owner's
			// context already validated.
			t := Template{}
			err := db.Where("name = ?", name).First(&t).Error
			if err != nil {
				return "", fmt.Errorf("include: template %q not found", name)
			}
			src := t.HTML
			if src == "" {
				src = t.Text
			}
			return executeTemplateWithDepth(src, data, depth+1)
		},
	}
}

func executeTemplateWithDepth(text string, data interface{}, depth int) (string, error) {
	buff := bytes.Buffer{}
	tmpl, err := template.New("template").Funcs(templateFuncs(data, depth)).Parse(text)
	if err != nil {
		return buff.String(), err
	}
	err = tmpl.Execute(&buff, data)
	return buff.String(), err
}

func ExecuteTemplate(text string, data interface{}) (string, error) {
	return executeTemplateWithDepth(text, data, 0)
}

// ValidationContext is used for validating templates and pages
type ValidationContext struct {
	FromAddress string
	BaseURL     string
}

func (vc ValidationContext) getFromAddress() string {
	return vc.FromAddress
}

func (vc ValidationContext) getBaseURL() string {
	return vc.BaseURL
}

func (vc ValidationContext) getEncryptionKey() string {
	return ""
}

// ValidateTemplate ensures that the provided text in the page or template
// uses the supported template variables correctly.
func ValidateTemplate(text string) error {
	vc := ValidationContext{
		FromAddress: "foo@bar.com",
		BaseURL:     "http://example.com",
	}
	td := Result{
		BaseRecipient: BaseRecipient{
			Email:     "foo@bar.com",
			FirstName: "Foo",
			LastName:  "Bar",
			Position:  "Test",
		},
		RId: "123456",
	}
	ptx, err := NewPhishingTemplateContext(vc, td.BaseRecipient, td.RId)
	if err != nil {
		return err
	}
	_, err = ExecuteTemplate(text, ptx)
	if err != nil {
		return err
	}
	return nil
}
