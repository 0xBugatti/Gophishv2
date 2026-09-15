package controllers

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gophish/gophish/config"
	ctx "github.com/gophish/gophish/context"
	"github.com/gophish/gophish/controllers/api"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/mailer"
	"github.com/gophish/gophish/models"
	"github.com/gophish/gomail"
	"github.com/gophish/gophish/util"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jordan-wright/unindexed"
)

// ErrInvalidRequest is thrown when a request with an invalid structure is
// received
var ErrInvalidRequest = errors.New("Invalid request")

// ErrCampaignComplete is thrown when an event is received for a campaign that
// has already been marked as complete.
var ErrCampaignComplete = errors.New("Event received on completed campaign")

// TransparencyResponse is the JSON response provided when a third-party
// makes a request to the transparency handler.
type TransparencyResponse struct {
	Server         string    `json:"server"`
	ContactAddress string    `json:"contact_address"`
	SendDate       time.Time `json:"send_date"`
}

// TransparencySuffix (when appended to a valid result ID), will cause Gophish
// to return a transparency response.
const TransparencySuffix = "+"

// botCIDRs contains well-known IP ranges used by email security scanners and
// image proxies. Clicks from these ranges are silently dropped (4.8).
var botCIDRs []*net.IPNet

// botUAs contains substrings that identify known security scanner user-agents.
var botUAs = []string{
	"Microsoft Office Existence Discovery",
	"Microsoft-WebDAV-MiniRedir",
	"msnbot",
	"AhrefsBot",
	"SemrushBot",
	"DotBot",
	"MJ12bot",
	"BLEXBot",
	// SafeLinks detonation sandbox UAs
	"Microsoft-IrmooBot",
	// Generic security scanners
	"Nessus",
	"Qualys",
	"Rapid7",
	"OpenVAS",
}

func init() {
	// Gmail Image Proxy, Microsoft SafeLinks, Google, and common security
	// scanner CIDR ranges (4.8).
	rawCIDRs := []string{
		"66.102.0.0/20",      // Google Image Proxy
		"209.85.128.0/17",    // Google
		"64.233.160.0/19",    // Google
		"72.14.192.0/18",     // Google
		"74.125.0.0/16",      // Google
		"40.94.0.0/16",       // Microsoft SafeLinks / ATP
		"40.107.0.0/16",      // Microsoft
		"52.96.0.0/14",       // Microsoft
		"104.47.0.0/17",      // Microsoft
	}
	for _, cidr := range rawCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err == nil {
			botCIDRs = append(botCIDRs, ipnet)
		}
	}
}

// isBotRequest returns true if the IP or User-Agent matches a known bot
// scanner signature (4.8).
func isBotRequest(ip, ua string) bool {
	parsed := net.ParseIP(ip)
	if parsed != nil {
		for _, cidr := range botCIDRs {
			if cidr.Contains(parsed) {
				return true
			}
		}
	}
	uaLower := strings.ToLower(ua)
	for _, bot := range botUAs {
		if strings.Contains(uaLower, strings.ToLower(bot)) {
			return true
		}
	}
	return false
}

// PhishingServerOption is a functional option that is used to configure the
// the phishing server
type PhishingServerOption func(*PhishingServer)

// PhishingServer is an HTTP server that implements the campaign event
// handlers, such as email open tracking, click tracking, and more.
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
}

// NewPhishingServer returns a new instance of the phishing server with
// provided options applied.
func NewPhishingServer(config config.PhishServer, options ...PhishingServerOption) *PhishingServer {
	defaultServer := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         config.ListenURL,
	}
	ps := &PhishingServer{
		server: defaultServer,
		config: config,
	}
	for _, opt := range options {
		opt(ps)
	}
	ps.registerRoutes()
	return ps
}

// WithContactAddress sets the contact address used by the transparency
// handlers
func WithContactAddress(addr string) PhishingServerOption {
	return func(ps *PhishingServer) {
		ps.contactAddress = addr
	}
}

// Start launches the phishing server, listening on the configured address.
func (ps *PhishingServer) Start() {
	if ps.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		ps.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(ps.config.CertPath, ps.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting phishing server at https://%s", ps.config.ListenURL)
		log.Fatal(ps.server.ListenAndServeTLS(ps.config.CertPath, ps.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting phishing server at http://%s", ps.config.ListenURL)
	log.Fatal(ps.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (ps *PhishingServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return ps.server.Shutdown(ctx)
}

// CreatePhishingRouter creates the router that handles phishing connections.
func (ps *PhishingServer) registerRoutes() {
	router := mux.NewRouter()
	fileServer := http.FileServer(unindexed.Dir("./static/endpoint/"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileServer))
	router.HandleFunc("/track", ps.TrackHandler)
	router.HandleFunc("/attachment_open", ps.AttachmentTrackHandler)
	router.HandleFunc("/robots.txt", ps.RobotsHandler)
	router.HandleFunc("/{path:.*}/track", ps.TrackHandler)
	router.HandleFunc("/{path:.*}/attachment_open", ps.AttachmentTrackHandler)
	router.HandleFunc("/{path:.*}/report", ps.ReportHandler)
	router.HandleFunc("/report", ps.ReportHandler)
	router.HandleFunc("/event", ps.CustomEventHandler)
	router.HandleFunc("/{path:.*}", ps.PhishHandler)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	phishHandler := gzipWrapper(router)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	phishHandler = handlers.ProxyHeaders(phishHandler)

	// Setup logging
	phishHandler = handlers.CombinedLoggingHandler(log.Writer(), phishHandler)
	ps.server.Handler = phishHandler
}

// CustomEventHandler handles arbitrary custom events triggered from landing pages
// (e.g. Word document opened, secondary link clicked). Requires ?userid=<rid>&title=<label>.
func (ps *PhishingServer) CustomEventHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	d := ctx.Get(r, "details").(models.EventDetails)
	err = rs.HandleCustomEvent(d)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TrackHandler tracks emails as they are opened, updating the status for the given Result
func (ps *PhishingServer) TrackHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		http.ServeFile(w, r, "static/images/pixel.png")
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	// Skip event recording for known bot/scanner requests (4.8).
	if isBot, _ := ctx.Get(r, "is_bot").(bool); !isBot {
		err = rs.HandleEmailOpened(d)
		if err != nil {
			log.Error(err)
		}
	}
	http.ServeFile(w, r, "static/images/pixel.png")
}

// AttachmentTrackHandler records attachment-open events, distinguishing them
// from regular link clicks (6.10). Triggered via a tracking URL embedded in
// rendered attachments with ?type=attachment.
func (ps *PhishingServer) AttachmentTrackHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	d := ctx.Get(r, "details").(models.EventDetails)
	if isBot, _ := ctx.Get(r, "is_bot").(bool); !isBot {
		err = rs.HandleAttachmentOpen(d)
		if err != nil {
			log.Error(err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReportHandler tracks emails as they are reported, updating the status for the given Result
func (ps *PhishingServer) ReportHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	w.Header().Set("Access-Control-Allow-Origin", "*") // To allow Chrome extensions (or other pages) to report a campaign without violating CORS
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailReport(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// PhishHandler handles incoming client connections and registers the associated actions performed
// (such as clicked link, etc.)
func (ps *PhishingServer) PhishHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Server", config.ServerName)
	var ptx models.PhishingTemplateContext
	// Check for a preview
	if preview, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		ptx, err = models.NewPhishingTemplateContext(&preview, preview.BaseRecipient, preview.RId)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
		p, err := models.GetPage(preview.PageId, preview.UserId)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
		renderPhishResponse(w, r, ptx, p)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	c := ctx.Get(r, "campaign").(models.Campaign)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	p, err := models.GetPage(c.PageId, c.UserId)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}
	isBot, _ := ctx.Get(r, "is_bot").(bool)
	switch {
	case r.Method == "GET":
		// Skip click recording for known security scanners (4.8).
		if !isBot {
			err = rs.HandleClickedLink(d)
			if err != nil {
				log.Error(err)
			}
		}
	case r.Method == "POST":
		err = rs.HandleFormSubmit(d)
		if err != nil {
			log.Error(err)
		}
		// If MFA is enabled on this page, intercept and show MFA verification page
		if p.EnableMFA {
			mfaCode := r.FormValue("mfa_code")
			if mfaCode != "" {
				if handleMFAVerification(w, r, rs, p, d, c, mfaCode) {
					return
				}
			} else {
				if handleMFAFlow(w, r, rs, p, d, c) {
					return
				}
			}
		}
	case r.Method == "PATCH":
		// Activity beacon: JS fires PATCH when real human interaction detected
		// (mousemove/click/keydown). Distinguishes humans from automated scanners.
		err = rs.HandleActivityInformation(d)
		if err != nil {
			log.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	ptx, err = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
	}
	renderPhishResponse(w, r, ptx, p)
}

// renderPhishResponse handles rendering the correct response to the phishing
// connection. This usually involves writing out the page HTML or redirecting
// the user to the correct URL.
func renderPhishResponse(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext, p models.Page) {
	if r.Method == "POST" {
		// Credential passthrough: forward captured form data to the real
		// site so the victim's login appears to succeed (3.11).
		if p.PassthroughURL != "" {
			go passthroughCredentials(p.PassthroughURL, r.Form)
		}

		// Multi-page flow: if a next page is configured, render it instead
		// of redirecting, enabling multi-step phishing flows (7.7).
		if p.NextPageId > 0 {
			nextPage, err := models.GetPageByID(p.NextPageId)
			if err == nil {
				html, err := models.ExecuteTemplate(nextPage.HTML, ptx)
				if err != nil {
					log.Error(err)
					http.NotFound(w, r)
					return
				}
				w.Write([]byte(html))
				return
			}
			log.Error(err)
		}

		// If an awareness page is configured, render it to inform the
		// target about the phishing simulation (7.5).
		if p.AwarenessPageHTML != "" {
			html, err := models.ExecuteTemplate(p.AwarenessPageHTML, ptx)
			if err != nil {
				log.Error(err)
				http.NotFound(w, r)
				return
			}
			w.Write([]byte(html))
			return
		}

		// Redirect after submit.
		switch p.RedirectMode {
		case "url":
			if p.RedirectURL != "" {
				redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
				if err != nil {
					log.Error(err)
					http.NotFound(w, r)
					return
				}
				http.Redirect(w, r, redirectURL, http.StatusFound)
				return
			}
			break
		case "html":
			html, err := models.ExecuteTemplate(p.RedirectHTML, ptx)
			if err != nil {
				log.Error(err)
				http.NotFound(w, r)
				return
			}
			w.Write([]byte(html))
			return
		default:
			log.Error("Redirect mode " + p.RedirectMode + " not found")
			http.NotFound(w, r)
			return
		}
	}
	// Render the landing page HTML.
	html, err := models.ExecuteTemplate(p.HTML, ptx)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}
	// Auto-submit countdown: inject a JS timer that automatically submits
	// the form after N seconds for passive victims who open but don't
	// interact (7.6).
	if p.AutoSubmitDelay > 0 {
		autoJS := fmt.Sprintf(`<script>setTimeout(function(){var f=document.forms[0];if(f)f.submit();},%d);</script>`, p.AutoSubmitDelay*1000)
		html = strings.Replace(html, "</body>", autoJS+"</body>", 1)
	}
	w.Write([]byte(html))
}

// passthroughCredentials POSTs captured form data to the configured real-site
// URL so the victim's login completes successfully (3.11). Runs in a goroutine
// so it doesn't block the response to the victim.
func passthroughCredentials(target string, form url.Values) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(target, form)
	if err != nil {
		log.Errorf("credential passthrough to %s failed: %v", target, err)
		return
	}
	resp.Body.Close()
}

// RobotsHandler prevents search engines, etc. from indexing phishing materials
func (ps *PhishingServer) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *\nDisallow: /")
}

// TransparencyHandler returns a TransparencyResponse for the provided result
// and campaign.
func (ps *PhishingServer) TransparencyHandler(w http.ResponseWriter, r *http.Request) {
	rs := ctx.Get(r, "result").(models.Result)
	tr := &TransparencyResponse{
		Server:         config.ServerName,
		SendDate:       rs.SendDate,
		ContactAddress: ps.contactAddress,
	}
	api.JSONResponse(w, tr, http.StatusOK)
}

// setupContext handles some of the administrative work around receiving a new
// request, such as checking the result ID, the campaign, etc.
func setupContext(r *http.Request) (*http.Request, error) {
	err := r.ParseForm()
	if err != nil {
		log.Error(err)
		return r, err
	}
	rid := r.Form.Get(models.RecipientParameter)
	if rid == "" {
		return r, ErrInvalidRequest
	}
	// Since we want to support the common case of adding a "+" to indicate a
	// transparency request, we need to take care to handle the case where the
	// request ends with a space, since a "+" is technically reserved for use
	// as a URL encoding of a space.
	if strings.HasSuffix(rid, " ") {
		// We'll trim off the space
		rid = strings.TrimRight(rid, " ")
		// Then we'll add the transparency suffix
		rid = fmt.Sprintf("%s%s", rid, TransparencySuffix)
	}
	// Finally, if this is a transparency request, we'll need to verify that
	// a valid rid has been provided, so we'll look up the result with a
	// trimmed parameter.
	id := strings.TrimSuffix(rid, TransparencySuffix)
	// Check to see if this is a preview or a real result
	if strings.HasPrefix(id, models.PreviewPrefix) {
		rs, err := models.GetEmailRequestByResultId(id)
		if err != nil {
			return r, err
		}
		r = ctx.Set(r, "result", rs)
		return r, nil
	}
	rs, err := models.GetResult(id)
	if err != nil {
		return r, err
	}
	c, err := models.GetCampaign(rs.CampaignId, rs.UserId)
	if err != nil {
		log.Error(err)
		return r, err
	}
	// Don't process events for completed campaigns
	if c.Status == models.CampaignComplete {
		return r, ErrCampaignComplete
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Handle post processing such as GeoIP
	err = rs.UpdateGeo(ip)
	if err != nil {
		log.Error(err)
	}
	d := models.EventDetails{
		Payload: r.Form,
		Browser: make(map[string]string),
	}
	ua := r.Header.Get("User-Agent")
	d.Browser["address"] = ip
	d.Browser["user-agent"] = ua

	r = ctx.Set(r, "rid", rid)
	r = ctx.Set(r, "result", rs)
	r = ctx.Set(r, "campaign", c)
	r = ctx.Set(r, "details", d)
	// Mark bot requests so handlers can skip event recording (4.8).
	r = ctx.Set(r, "is_bot", isBotRequest(ip, ua))
	return r, nil
}
// handleMFAFlow handles the MFA verification flow for landing pages
// Returns true if the MFA flow handled the response (caller should return early)
// Returns false if MFA should be skipped (no phone available, etc.)
func handleMFAFlow(w http.ResponseWriter, r *http.Request, rs models.Result, p models.Page, d models.EventDetails, c models.Campaign) bool {
	// Check if this is an MFA code submission (has mfa_code field)
	mfaCode := r.FormValue("mfa_code")
	if mfaCode != "" {
		// This is an MFA verification attempt
		return handleMFAVerification(w, r, rs, p, d, c, mfaCode)
	}

	// Determine MFA delivery type
	mfaType := p.MFAType
	if mfaType == "" {
		mfaType = "sms"
	}

	// Generate MFA code (all types share the same code generation)
	codeLength := p.MFACodeLength
	if codeLength <= 0 {
		codeLength = 6
	}
	codeType := p.MFACodeType
	if codeType == "" {
		codeType = models.MFACodeTypeNumeric
	}

	code, err := models.GenerateMFACode(codeLength, codeType)
	if err != nil {
		log.Errorf("Failed to generate MFA code for rid %s: %v", rs.RId, err)
		return false
	}

	// Delivery target — phone for SMS, email for email OTP, empty for TOTP
	deliveryTarget := ""

	switch mfaType {
	case "totp":
		// TOTP / Authenticator App — no delivery needed; just capture whatever code the target enters
		_, err = models.SaveMFACode(rs.RId, "", "totp")
		if err != nil {
			log.Errorf("Failed to save TOTP MFA placeholder for rid %s: %v", rs.RId, err)
			return false
		}
		mfaSentDetails := models.EventDetails{Payload: make(map[string][]string), Browser: d.Browser}
		mfaSentDetails.Payload["mfa_type"] = []string{"totp"}
		if recordErr := rs.HandleMFACodeSent(mfaSentDetails); recordErr != nil {
			log.Errorf("Failed to record TOTP MFA event for rid %s: %v", rs.RId, recordErr)
		}

	case "email":
		// Email OTP — deliver code to target's email address via chosen SMTP profile
		email := rs.Email
		if email == "" {
			email = r.FormValue("email")
		}
		if email == "" {
			log.Warnf("MFA email enabled for page %d but no email for rid %s, skipping MFA", p.Id, rs.RId)
			return false
		}
		if p.MFAEmailProfileId == 0 {
			log.Warnf("MFA email enabled for page %d but no SMTP profile configured, skipping MFA", p.Id)
			return false
		}
		smtpProfile, smtpErr := models.GetSMTP(p.MFAEmailProfileId, p.UserId)
		if smtpErr != nil {
			log.Errorf("Failed to get SMTP profile %d for MFA email: %v", p.MFAEmailProfileId, smtpErr)
			return false
		}
		deliveryTarget = email
		_, err = models.SaveMFACode(rs.RId, code, deliveryTarget)
		if err != nil {
			log.Errorf("Failed to save MFA email code for rid %s: %v", rs.RId, err)
			return false
		}
		message := models.GenerateMFAMessage(p.MFAMessage, code)
		fromSender := p.MFAFrom
		if fromSender == "" {
			fromSender = smtpProfile.FromAddress
		}
		err = sendMFAEmail(smtpProfile, email, fromSender, message, code)
		if err != nil {
			log.Errorf("Failed to send MFA email to %s for rid %s: %v", email, rs.RId, err)
		} else {
			log.Infof("MFA email sent to %s for rid %s", email, rs.RId)
			mfaSentDetails := models.EventDetails{Payload: make(map[string][]string), Browser: d.Browser}
			mfaSentDetails.Payload["mfa_email"] = []string{email}
			mfaSentDetails.Payload["mfa_type"] = []string{"email"}
			if recordErr := rs.HandleMFACodeSent(mfaSentDetails); recordErr != nil {
				log.Errorf("Failed to record MFA email sent event for rid %s: %v", rs.RId, recordErr)
			}
		}

	default: // "sms"
		// SMS OTP — existing behaviour
		phone := rs.Phone
		if phone == "" {
			phone = r.FormValue("phone")
			if phone == "" {
				phone = r.FormValue("Phone")
			}
		}
		if phone == "" {
			log.Warnf("MFA SMS enabled for page %d but no phone for rid %s, skipping MFA", p.Id, rs.RId)
			return false
		}
		if p.MFASMSProfileId == 0 {
			log.Warnf("MFA SMS enabled for page %d but no SMS profile configured, skipping MFA", p.Id)
			return false
		}
		smsProfile, smsErr := models.GetSMS(p.MFASMSProfileId, p.UserId)
		if smsErr != nil {
			log.Errorf("Failed to get SMS profile %d for MFA: %v", p.MFASMSProfileId, smsErr)
			return false
		}
		deliveryTarget = phone
		_, err = models.SaveMFACode(rs.RId, code, deliveryTarget)
		if err != nil {
			log.Errorf("Failed to save MFA SMS code for rid %s: %v", rs.RId, err)
			return false
		}
		message := models.GenerateMFAMessage(p.MFAMessage, code)
		fromSender := p.MFAFrom
		if fromSender == "" {
			fromSender = smsProfile.From
		}
		err = sendMFASMS(smsProfile, phone, message, fromSender)
		if err != nil {
			log.Errorf("Failed to send MFA SMS to %s for rid %s: %v", phone, rs.RId, err)
		// Record the failure as a visible event in campaign results
		if recordErr := rs.HandleMFACodeSendError(err); recordErr != nil {
			log.Errorf("Failed to record MFA send error event for rid %s: %v", rs.RId, recordErr)
		}
		// Still show the MFA page even if SMS failed - user can retry
	} else {
		log.Infof("MFA code sent to %s for rid %s (from: %s)", phone, rs.RId, fromSender)
		// Only record "MFA Code Sent" when the SMS was actually delivered to the provider
		mfaSentDetails := models.EventDetails{
			Payload: make(map[string][]string),
			Browser: d.Browser,
		}
		mfaSentDetails.Payload["mfa_phone"] = []string{phone}
		mfaSentDetails.Payload["mfa_from"] = []string{fromSender}
		mfaSentDetails.Payload["mfa_type"] = []string{"sms"}
		if recordErr := rs.HandleMFACodeSent(mfaSentDetails); recordErr != nil {
			log.Errorf("Failed to record MFA code sent event for rid %s: %v", rs.RId, recordErr)
		}
	}
	} // end switch mfaType

	// Show the MFA verification page
	if p.MFAInjectPage {
		// Use the injected MFA page (custom if set, otherwise default)
		mfaHTML := models.RenderMFAPage(p.MFAPageHTML, rs.RId, "")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mfaHTML))
	} else {
		// Re-render the original page - user's JS should handle showing MFA input
		// We'll add a special header or hidden field to indicate MFA is pending
		var ptx models.PhishingTemplateContext
		if c.Type == "sms" {
			stx, _ := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           stx.URL,
				TrackingURL:   stx.TrackingURL,
				Tracker:       "<img alt='' style='display: none' src='" + stx.TrackingURL + "'/>",
				From:          stx.From,
				BaseURL:       stx.BaseURL,
			}
		} else if c.Type == "generic" {
			trackingURL, _ := models.ExecuteTemplate(c.URL, nil)
			if trackingURL == "" {
				trackingURL = c.URL
			}
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           c.URL,
				TrackingURL:   trackingURL + "/track?rid=" + rs.RId,
				Tracker:       "<img alt='' style='display: none' src='" + trackingURL + "/track?rid=" + rs.RId + "'/>",
				From:          "",
				BaseURL:       c.URL,
			}
		} else {
			ptx, _ = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
		}

		// Add a hidden indicator that MFA is pending
		html, err := models.ExecuteTemplate(p.HTML, ptx)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return true
		}

		// Inject MFA pending indicator before </body>
		mfaIndicator := `<input type="hidden" id="mfa_pending" value="true"><script>window.mfaPending=true;</script>`
		html = strings.Replace(html, "</body>", mfaIndicator+"</body>", 1)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}

	return true
}

// handleMFAVerification handles the verification of an MFA code
func handleMFAVerification(w http.ResponseWriter, r *http.Request, rs models.Result, p models.Page, d models.EventDetails, c models.Campaign, submittedCode string) bool {
	// Verify the MFA code
	verified, err := models.VerifyMFACode(rs.RId, submittedCode)
	if err != nil {
		log.Warnf("MFA verification error for rid %s: %v", rs.RId, err)
	}

	if verified {
		// Mark the code as verified
		models.MarkMFACodeVerified(rs.RId)

		log.Infof("MFA code verified successfully for rid %s", rs.RId)

		// Record the MFA verification event
		mfaVerifiedDetails := models.EventDetails{
			Payload: make(map[string][]string),
			Browser: d.Browser,
		}
		mfaVerifiedDetails.Payload["mfa_verified"] = []string{"true"}
		err = rs.HandleMFACodeVerified(mfaVerifiedDetails)
		if err != nil {
			log.Error(err)
		}

		// Proceed to redirect or show the final page
		if p.RedirectURL != "" {
			var ptx models.PhishingTemplateContext
			if c.Type == "sms" {
				stx, _ := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
				ptx = models.PhishingTemplateContext{
					BaseRecipient: rs.BaseRecipient,
					RId:           rs.RId,
					URL:           stx.URL,
					TrackingURL:   stx.TrackingURL,
					From:          stx.From,
					BaseURL:       stx.BaseURL,
				}
			} else if c.Type == "generic" {
				ptx = models.PhishingTemplateContext{
					BaseRecipient: rs.BaseRecipient,
					RId:           rs.RId,
					URL:           c.URL,
					BaseURL:       c.URL,
				}
			} else {
				ptx, _ = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
			}

			redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
			if err != nil {
				log.Error(err)
				http.NotFound(w, r)
				return true
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return true
		}

		// No redirect, just show success or re-render page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Verified</title></head><body><h1>Verification successful</h1></body></html>`))
		return true
	}

	// Verification failed
	log.Warnf("MFA code verification failed for rid %s", rs.RId)

	// Record the MFA failure event
	mfaFailedDetails := models.EventDetails{
		Payload: make(map[string][]string),
		Browser: d.Browser,
	}
	mfaFailedDetails.Payload["mfa_verified"] = []string{"false"}
	mfaFailedDetails.Payload["mfa_code_submitted"] = []string{submittedCode}
	err = rs.HandleMFACodeFailed(mfaFailedDetails)
	if err != nil {
		log.Error(err)
	}

	// Show the MFA page again with an error
	if p.MFAInjectPage {
		mfaHTML := models.RenderMFAPage(p.MFAPageHTML, rs.RId, "Invalid verification code. Please try again.")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mfaHTML))
	} else {
		// Re-render page with error indicator
		var ptx models.PhishingTemplateContext
		if c.Type == "sms" {
			stx, _ := models.NewSMSTemplateContext(&c, rs.BaseRecipient, rs.RId)
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           stx.URL,
				TrackingURL:   stx.TrackingURL,
				From:          stx.From,
				BaseURL:       stx.BaseURL,
			}
		} else if c.Type == "generic" {
			ptx = models.PhishingTemplateContext{
				BaseRecipient: rs.BaseRecipient,
				RId:           rs.RId,
				URL:           c.URL,
				BaseURL:       c.URL,
			}
		} else {
			ptx, _ = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
		}

		html, err := models.ExecuteTemplate(p.HTML, ptx)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return true
		}

		// Inject MFA error indicator
		mfaIndicator := `<input type="hidden" id="mfa_error" value="Invalid code"><script>window.mfaError="Invalid code";</script>`
		html = strings.Replace(html, "</body>", mfaIndicator+"</body>", 1)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	}

	return true
}

// sendMFAEmail sends an MFA OTP code to the target's email address via the configured SMTP profile.
func sendMFAEmail(smtp models.SMTP, toEmail, fromAddress, body, code string) error {
	dialer, err := smtp.GetDialer()
	if err != nil {
		return fmt.Errorf("failed to get SMTP dialer for MFA email: %w", err)
	}
	sender, err := dialer.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial SMTP for MFA email: %w", err)
	}
	defer sender.Close()

	subject := "Your verification code"
	if body == "" {
		body = "Your verification code is: " + code
	}
	htmlBody := "<html><body><p>" + body + "</p></body></html>"

	msg := gomail.NewMessage()
	msg.SetHeader("From", fromAddress)
	msg.SetHeader("To", toEmail)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", body)
	msg.AddAlternative("text/html", htmlBody)

	return gomail.Send(sender, msg)
}

// sendMFASMS sends an MFA code via SMS using the provided SMS profile
// fromSender allows overriding the sender ID (if empty, uses profile default)
func sendMFASMS(smsProfile models.SMS, phone string, message string, fromSender string) error {
	// Ensure phone number is in E.164 format (starts with +)
	if phone != "" && !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}

	// Get the SMS dialer
	dialer, err := mailer.GetSMSDialer(smsProfile.Provider, smsProfile.ProviderConfig)
	if err != nil {
		return fmt.Errorf("failed to get SMS dialer: %w", err)
	}

	// Dial to get the provider
	provider, err := dialer.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial SMS provider: %w", err)
	}
	defer provider.Close()

	// Send the SMS with the specified sender
	err = provider.Send(fromSender, phone, message)
	if err != nil {
		return fmt.Errorf("failed to send SMS: %w", err)
	}

	return nil
}
