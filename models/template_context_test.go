package models

import (
	"net/url"

	"github.com/gophish/gophish/evilginx"
	check "gopkg.in/check.v1"
)

type mockTemplateContext struct {
	URL         string
	FromAddress string
}

func (m mockTemplateContext) getFromAddress() string {
	return m.FromAddress
}

func (m mockTemplateContext) getBaseURL() string {
	return m.URL
}

func (m mockTemplateContext) getEncryptionKey() string {
	return ""
}

// decodeEvilginxURL extracts the plain query parameters from an evilginx-obfuscated URL.
func decodeEvilginxURL(rawURL string) (map[string]string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	for _, v := range u.Query() {
		params, ok, _ := evilginx.ExtractPhishUrlParams(v[0], "")
		if ok {
			return params, nil
		}
	}
	return map[string]string{}, nil
}

func (s *ModelsSuite) TestNewTemplateContext(c *check.C) {
	r := Result{
		BaseRecipient: BaseRecipient{
			FirstName: "Foo",
			LastName:  "Bar",
			Email:     "foo@bar.com",
		},
		RId: "1234567",
	}
	ctx := mockTemplateContext{
		URL:         "http://example.com",
		FromAddress: "From Address <from@example.com>",
	}
	got, err := NewPhishingTemplateContext(ctx, r.BaseRecipient, r.RId)
	c.Assert(err, check.Equals, nil)
	c.Assert(got.BaseURL, check.Equals, ctx.URL)
	c.Assert(got.BaseRecipient, check.DeepEquals, r.BaseRecipient)
	c.Assert(got.From, check.Equals, "From Address")
	c.Assert(got.RId, check.Equals, r.RId)
	c.Assert(got.FromEmail, check.Equals, "from@example.com")
	c.Assert(got.Domain, check.Equals, "bar.com") // domain of recipient email foo@bar.com

	// URL is evilginx-obfuscated; decode and verify the recipient parameter
	urlParams, err := decodeEvilginxURL(got.URL)
	c.Assert(err, check.Equals, nil)
	c.Assert(urlParams[RecipientParameter], check.Equals, r.RId)

	// TrackingURL also carries the tracking flag
	trackParams, err := decodeEvilginxURL(got.TrackingURL)
	c.Assert(err, check.Equals, nil)
	c.Assert(trackParams[RecipientParameter], check.Equals, r.RId)
	c.Assert(trackParams["o"], check.Equals, "track")
}
