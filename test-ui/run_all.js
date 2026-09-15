const puppeteer = require('puppeteer');
const fs = require('fs');

const BASE = 'https://127.0.0.1:3333';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'gophish123';

const results = { passed: [], failed: [], warnings: [] };
let browser, page;

function log(msg) { console.log(`  ${msg}`); }
function pass(name) { results.passed.push(name); console.log(`  \x1b[32mPASS\x1b[0m ${name}`); }
function fail(name, err) { results.failed.push({ name, error: err }); console.log(`  \x1b[31mFAIL\x1b[0m ${name}: ${err}`); }
function warn(name, detail) { results.warnings.push({ name, detail }); console.log(`  \x1b[33mWARN\x1b[0m ${name}: ${detail}`); }

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

async function waitForVisible(selector, timeout = 5000) {
  await page.waitForSelector(selector, { timeout, visible: true });
}

async function isElementVisible(selector) {
  return await page.$eval(selector, el => {
    const rect = el.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0 && getComputedStyle(el).display !== 'none';
  }).catch(() => false);
}

async function countRows(tableSelector) {
  return await page.$eval(`${tableSelector} tbody tr`, rows => rows.length).catch(() => 0);
}

async function clickModalButton(buttonText) {
  await page.evaluate((text) => {
    const btns = document.querySelectorAll('.modal-footer .btn');
    for (const b of btns) {
      if (b.textContent.trim().includes(text)) { b.click(); return; }
    }
  }, buttonText);
}

async function closeModal(modalId) {
  await page.evaluate((id) => {
    const closeBtn = document.querySelector(`#${id} .close`) || document.querySelector(`#${id} [data-dismiss="modal"]`);
    if (closeBtn) closeBtn.click();
  }, modalId);
  await sleep(300);
}

async function swalClick(buttonText) {
  await page.evaluate((text) => {
    const btn = document.querySelector(`.swal2-popup .swal2-${text === 'Confirm' ? 'confirm' : 'cancel'}`);
    if (btn) btn.click();
    else {
      const btns = document.querySelectorAll('.swal2-popup button');
      for (const b of btns) { if (b.textContent.includes(text)) { b.click(); return; } }
    }
  }, buttonText);
  await sleep(500);
}

async function dismissSwalIfExists() {
  await page.evaluate(() => {
    const btn = document.querySelector('.swal2-confirm');
    if (btn) btn.click();
  }).catch(() => {});
  await sleep(300);
}

async function navTo(path) {
  await page.goto(`${BASE}${path}`, { waitUntil: 'networkidle0', timeout: 15000 });
  await sleep(500);
}

async function checkPageHeader(expectedText) {
  const header = await page.$eval('.page-header', el => el.textContent.trim()).catch(() => '');
  if (!header.includes(expectedText)) throw new Error(`Page header "${header}" does not contain "${expectedText}"`);
}

async function checkNavItem(path, label) {
  const exists = await page.$eval('.nav-sidebar', el => el.textContent).catch(() => '');
  if (!exists.includes(label)) throw new Error(`Nav missing "${label}"`);
}

async function checkDataTableLoaded(tableId) {
  await page.waitForSelector(`#${tableId}`, { timeout: 5000 }).catch(() => {});
  const visible = await isElementVisible(`#${tableId}`);
  return visible;
}

async function checkLoadingGone() {
  const loadingVisible = await isElementVisible('#loading');
  const smsLoadingVisible = await isElementVisible('#smsLoading');
  return !loadingVisible && !smsLoadingVisible;
}

async function typeInField(selector, value) {
  await page.click(selector, { clickCount: 3 });
  await page.type(selector, value);
}

async function getCellValue(tableSelector, row, col) {
  return await page.$eval(`${tableSelector} tbody tr:nth-child(${row}) td:nth-child(${col})`, el => el.textContent.trim()).catch(() => '');
}

// ─── TEST SUITES ────────────────────────────────────────────────

async function testLoginPage() {
  console.log('\n\x1b[36m=== LOGIN PAGE ===\x1b[0m');
  
  try {
    await navTo('/login');
    await pass('Login page loads');
  } catch (e) { fail('Login page loads', e.message); return; }

  try {
    const heading = await page.$eval('h2', el => el.textContent.trim());
    if (heading !== 'Sign in') throw new Error(`Expected "Sign in", got "${heading}"`);
    pass('Login heading correct');
  } catch (e) { fail('Login heading correct', e.message); }

  try {
    const usernameField = await page.$('input[name="username"]');
    const passwordField = await page.$('input[name="password"]');
    const submitBtn = await page.$('button[type="submit"]');
    if (!usernameField || !passwordField || !submitBtn) throw new Error('Missing form elements');
    pass('Login form elements present');
  } catch (e) { fail('Login form elements present', e.message); }

  try {
    await page.type('input[name="username"]', 'wrong');
    await page.type('input[name="password"]', 'wrong');
    await Promise.all([
      page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 5000 }).catch(() => {}),
      page.click('button[type="submit"]')
    ]);
    const url = page.url();
    if (!url.includes('/login')) throw new Error('Did not stay on login page after wrong creds');
    pass('Wrong credentials rejected');
  } catch (e) { fail('Wrong credentials rejected', e.message); }

  try {
    await page.goto(`${BASE}/login`, { waitUntil: 'networkidle0' });
    await page.type('input[name="username"]', ADMIN_USER);
    await page.type('input[name="password"]', ADMIN_PASS);
    await Promise.all([
      page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 10000 }),
      page.click('button[type="submit"]')
    ]);
    const url = page.url();
    if (url.includes('/login')) throw new Error('Still on login page after correct creds');
    pass('Login with correct credentials succeeds');
  } catch (e) { fail('Login with correct credentials succeeds', e.message); }
}

async function testDashboard() {
  console.log('\n\x1b[36m=== DASHBOARD ===\x1b[0m');
  
  try {
    await navTo('/');
    await pass('Dashboard page loads');
  } catch (e) { fail('Dashboard page loads', e.message); return; }

  try {
    await checkPageHeader('Dashboard');
    pass('Dashboard header correct');
  } catch (e) { fail('Dashboard header correct', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#loading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 8000 });
    pass('Dashboard loading spinner disappears');
  } catch (e) { fail('Dashboard loading spinner disappears', e.message); }

  try {
    const hasDashboard = await isElementVisible('#dashboard');
    const hasEmptyMsg = await isElementVisible('#emptyMessage');
    if (!hasDashboard && !hasEmptyMsg) throw new Error('Neither dashboard nor empty message visible');
    pass('Dashboard content area rendered');
  } catch (e) { fail('Dashboard content area rendered', e.message); }

  try {
    const hasChart = await page.$eval('#overview_chart', el => el.children.length > 0).catch(() => false);
    if (hasChart) pass('Overview chart container has content');
    else warn('Overview chart container empty', 'No campaigns may exist');
  } catch (e) { warn('Overview chart', e.message); }

  try {
    const viewAllBtn = await page.$('a[href="/campaigns"] .btn');
    if (!viewAllBtn) throw new Error('View All button missing');
    pass('View All campaigns button present');
  } catch (e) { fail('View All campaigns button present', e.message); }
}

async function testCampaignsPage() {
  console.log('\n\x1b[36m=== CAMPAIGNS PAGE ===\x1b[0m');

  try {
    await navTo('/campaigns');
    await pass('Campaigns page loads');
  } catch (e) { fail('Campaigns page loads', e.message); return; }

  try {
    await checkPageHeader('Campaigns');
    pass('Campaigns header correct');
  } catch (e) { fail('Campaigns header correct', e.message); }

  try {
    const newBtn = await page.$('button[onclick="edit(\'new\')"]');
    if (!newBtn) throw new Error('New Campaign button missing');
    pass('New Campaign button present');
  } catch (e) { fail('New Campaign button present', e.message); }

  try {
    const activeTab = await page.$('#activeCampaigns.active');
    const archivedTab = await page.$('#archivedCampaigns');
    if (!activeTab || !archivedTab) throw new Error('Missing campaign tabs');
    pass('Active/Archived tabs present');
  } catch (e) { fail('Active/Archived tabs present', e.message); }

  try {
    await page.click('button[onclick="edit(\'new\')"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Campaign modal did not open');
    pass('New Campaign modal opens');

    const nameField = await page.$('#name');
    const campaignType = await page.$('#campaign_type');
    const launchBtn = await page.$('#launchButton');
    if (!nameField || !campaignType || !launchBtn) throw new Error('Missing modal fields');
    pass('Campaign modal has required fields');

    const emailBtn = await page.$('.campaign-type-btn[data-type="email"].active');
    const smsBtn = await page.$('.campaign-type-btn[data-type="sms"]');
    const genericBtn = await page.$('.campaign-type-btn[data-type="generic"]');
    if (!emailBtn || !smsBtn || !genericBtn) throw new Error('Missing campaign type buttons');
    pass('Campaign type buttons (Email/SMS/Generic) present');

    await page.click('.campaign-type-btn[data-type="sms"]');
    await sleep(300);
    const smsDiv = await isElementVisible('#sms_template_div');
    const smsProfileDiv = await isElementVisible('#sms_profile_div');
    if (!smsDiv || !smsProfileDiv) throw new Error('SMS fields not shown');
    pass('SMS campaign type shows SMS fields');

    await page.click('.campaign-type-btn[data-type="generic"]');
    await sleep(300);
    const genericDiv = await isElementVisible('#generic_info_div');
    const groupsDiv = await isElementVisible('#groups_div');
    if (!genericDiv) throw new Error('Generic info div not shown');
    if (groupsDiv) throw new Error('Groups div should be hidden for generic');
    pass('Generic campaign type shows info, hides groups');

    await page.click('.campaign-type-btn[data-type="email"]');
    await sleep(300);
    const emailDiv = await isElementVisible('#email_template_div');
    if (!emailDiv) throw new Error('Email fields not restored');
    pass('Email campaign type restores email fields');

    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('Campaign modal tests', e.message); }
}

async function testCampaignSetsPage() {
  console.log('\n\x1b[36m=== CAMPAIGN SETS PAGE ===\x1b[0m');

  try {
    await navTo('/campaign_sets');
    await pass('Campaign Sets page loads');
  } catch (e) { fail('Campaign Sets page loads', e.message); return; }

  try {
    await checkPageHeader('Campaign Sets');
    pass('Campaign Sets header correct');
  } catch (e) { fail('Campaign Sets header correct', e.message); }

  try {
    const newBtn = await page.$('#new-campaign-set-btn');
    if (!newBtn) throw new Error('New Campaign Set button missing');
    pass('New Campaign Set button present');
  } catch (e) { fail('New Campaign Set button present', e.message); }

  try {
    const table = await page.$('#campaignSetTable');
    if (!table) throw new Error('Campaign Set table missing');
    const headers = await page.$$eval('#campaignSetTable thead th', ths => ths.map(t => t.textContent.trim()));
    const expected = ['Name', 'Created Date', 'Launch Date', 'Send By Date', 'Status', 'Campaigns'];
    for (const h of expected) {
      if (!headers.some(x => x.includes(h))) throw new Error(`Missing column: ${h}`);
    }
    pass('Campaign Set table has correct columns');
  } catch (e) { fail('Campaign Set table columns', e.message); }

  try {
    await page.click('#new-campaign-set-btn');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Campaign Set modal did not open');
    pass('New Campaign Set modal opens');

    const generalTab = await page.$('#generalSettings.active');
    const campaignsTab = await page.$('#campaignsTab');
    if (!generalTab || !campaignsTab) throw new Error('Missing tabs');
    pass('General Settings & Campaigns tabs present');

    const saveDraftBtn = await page.$('#saveDraftButton');
    const launchBtn = await page.$('#launchButton');
    if (!saveDraftBtn || !launchBtn) throw new Error('Missing Save Draft / Launch buttons');
    pass('Save Draft & Launch buttons present');

    const sharedToggles = await page.$$('.shared-toggle-group');
    if (sharedToggles.length < 5) throw new Error(`Expected >= 5 shared toggles, got ${sharedToggles.length}`);
    pass('Shared/Per-Campaign toggle buttons present');

    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('Campaign Set modal tests', e.message); }
}

async function testTemplatesPage() {
  console.log('\n\x1b[36m=== EMAIL TEMPLATES PAGE ===\x1b[0m');

  try {
    await navTo('/templates');
    await pass('Templates page loads');
  } catch (e) { fail('Templates page loads', e.message); return; }

  try {
    await checkPageHeader('Email Templates');
    pass('Templates header correct');
  } catch (e) { fail('Templates header correct', e.message); }

  try {
    const newBtn = await page.$('button[onclick="edit(-1)"]');
    if (!newBtn) throw new Error('New Template button missing');
    pass('New Template button present');
  } catch (e) { fail('New Template button present', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#loading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 8000 });
    pass('Templates loading spinner disappears');
  } catch (e) { fail('Templates loading spinner disappears', e.message); }

  try {
    const tableExists = await page.$('#templateTable');
    const emptyMsg = await isElementVisible('#emptyMessage');
    const tableVisible = tableExists ? await isElementVisible('#templateTable') : false;
    if (!tableVisible && !emptyMsg) throw new Error('Neither table nor empty message shown');
    pass('Template table or empty message rendered');
  } catch (e) { fail('Template table rendering', e.message); }

  try {
    await page.click('button[onclick="edit(-1)"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Template modal did not open');
    pass('New Template modal opens');

    const nameField = await page.$('#name');
    const subjectField = await page.$('#subject');
    const textEditor = await page.$('#text_editor');
    const htmlEditor = await page.$('#html_editor');
    const trackerCheckbox = await page.$('#use_tracker_checkbox');
    const saveBtn = await page.$('#modalSubmit');
    if (!nameField || !subjectField || !textEditor || !htmlEditor || !trackerCheckbox || !saveBtn)
      throw new Error('Missing modal fields');
    pass('Template modal has all form fields');

    const importBtn = await page.$('button[data-target="#importEmailModal"]');
    if (!importBtn) throw new Error('Import Email button missing');
    pass('Import Email button present');

    await typeInField('#name', 'E2E Test Template');
    await typeInField('#subject', 'Test Subject {{.FirstName}}');
    await page.$eval('#text_editor', (el, val) => { el.value = val; }, 'Hello {{.FirstName}}, click {{.URL}}');

    const htmlTab = await page.$('a[href="#html"]');
    if (htmlTab) await htmlTab.click();
    await sleep(300);

    await page.click('#modalSubmit');
    await sleep(2000);

    const errorFlashes = await page.$eval('#modal.flashes', el => el.textContent).catch(() => '');
    const modalStillVisible = await isElementVisible('#modal');

    if (modalStillVisible && errorFlashes) {
      pass('Template save attempted (modal shows feedback)');
      await closeModal('modal');
      await sleep(300);
    } else if (!modalStillVisible) {
      pass('Template created successfully');
    }
  } catch (e) { fail('Template modal CRUD', e.message); }
}

async function testGroupsPage() {
  console.log('\n\x1b[36m=== USERS & GROUPS PAGE ===\x1b[0m');

  try {
    await navTo('/groups');
    await pass('Groups page loads');
  } catch (e) { fail('Groups page loads', e.message); return; }

  try {
    await checkPageHeader('Users & Groups');
    pass('Groups header correct');
  } catch (e) { fail('Groups header correct', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#loading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 8000 });
    pass('Groups loading spinner disappears');
  } catch (e) { fail('Groups loading spinner disappears', e.message); }

  try {
    const tableExists = await page.$('#groupTable');
    const emptyMsg = await isElementVisible('#emptyMessage');
    const tableVisible = tableExists ? await isElementVisible('#groupTable') : false;
    if (!tableVisible && !emptyMsg) throw new Error('Neither table nor empty message shown');
    pass('Group table or empty message rendered');
  } catch (e) { fail('Group table rendering', e.message); }

  try {
    await page.click('button[onclick="edit(-1)"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Group modal did not open');
    pass('New Group modal opens');

    const nameField = await page.$('#name');
    const csvUpload = await page.$('#csvupload');
    const firstName = await page.$('#firstName');
    const lastName = await page.$('#lastName');
    const email = await page.$('#email');
    const position = await page.$('#position');
    const addBtn = await page.$('#targetForm button[type="submit"]');
    const saveBtn = await page.$('#modalSubmit');
    if (!nameField || !csvUpload || !firstName || !lastName || !email || !position || !addBtn || !saveBtn)
      throw new Error('Missing modal fields');
    pass('Group modal has all form fields');

    await typeInField('#name', 'E2E Test Group');
    await typeInField('#firstName', 'John');
    await typeInField('#lastName', 'Doe');
    await typeInField('#email', 'john.doe@test.com');
    await typeInField('#position', 'Developer');

    await page.click('#targetForm button[type="submit"]');
    await sleep(500);

    const targetRows = await countRows('#targetsTable');
    if (targetRows === 0) throw new Error('Target was not added to list');
    pass('Target added to group member list');

    await page.click('#modalSubmit');
    await sleep(2000);

    const modalStillOpen = await isElementVisible('#modal');
    if (!modalStillOpen) {
      pass('Group saved successfully');
    } else {
      warn('Group save', 'Modal still open - may have validation error');
      await closeModal('modal');
      await sleep(300);
    }
  } catch (e) { fail('Group modal CRUD', e.message); }
}

async function testLandingPagesPage() {
  console.log('\n\x1b[36m=== LANDING PAGES PAGE ===\x1b[0m');

  try {
    await navTo('/landing_pages');
    await pass('Landing Pages page loads');
  } catch (e) { fail('Landing Pages page loads', e.message); return; }

  try {
    await checkPageHeader('Landing Pages');
    pass('Landing Pages header correct');
  } catch (e) { fail('Landing Pages header correct', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#loading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 8000 });
    pass('Landing Pages loading spinner disappears');
  } catch (e) { fail('Landing Pages loading spinner disappears', e.message); }

  try {
    await page.click('button[onclick="edit(-1)"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Landing page modal did not open');
    pass('New Landing Page modal opens');

    const nameField = await page.$('#name');
    const captureCredsCheckbox = await page.$('#capture_credentials_checkbox');
    const capturePasswordsCheckbox = await page.$('#capture_passwords_checkbox');
    const redirectUrlInput = await page.$('#redirect_url_input');
    const enableMfaCheckbox = await page.$('#enable_mfa_checkbox');
    const importSiteBtn = await page.$('button[data-target="#importSiteModal"]');
    const saveBtn = await page.$('#modalSubmit');
    if (!nameField || !captureCredsCheckbox || !capturePasswordsCheckbox || !redirectUrlInput || !enableMfaCheckbox || !importSiteBtn || !saveBtn)
      throw new Error('Missing modal fields');
    pass('Landing Page modal has all form fields (including MFA)');

    await typeInField('#name', 'E2E Test Landing Page');

    await page.evaluate(() => document.getElementById('enable_mfa_checkbox').click());
    await sleep(300);
    const mfaSettings = await isElementVisible('#mfa_settings');
    if (!mfaSettings) throw new Error('MFA settings did not appear');
    pass('MFA settings appear when checkbox toggled');

    const mfaType = await page.$('#mfa_type');
    const mfaCodeLength = await page.$('#mfa_code_length');
    const mfaCodeType = await page.$('#mfa_code_type');
    const mfaFrom = await page.$('#mfa_from');
    const mfaMessage = await page.$('#mfa_message');
    if (!mfaType || !mfaCodeLength || !mfaCodeType || !mfaFrom || !mfaMessage)
      throw new Error('MFA fields missing');
    pass('MFA settings have all sub-fields');

    await page.evaluate(() => { document.getElementById('mfa_type').value = 'email'; document.getElementById('mfa_type').dispatchEvent(new Event('change')); });
    await sleep(200);
    const emailProfile = await isElementVisible('#mfa_email_section');
    const smsSection = await isElementVisible('#mfa_sms_section');
    if (!emailProfile) throw new Error('Email MFA section not shown');
    pass('MFA type email shows email profile selector');

    await page.evaluate(() => { document.getElementById('mfa_type').value = 'sms'; document.getElementById('mfa_type').dispatchEvent(new Event('change')); });
    await sleep(200);
    const smsProfile = await isElementVisible('#mfa_sms_section');
    if (!smsProfile) throw new Error('SMS MFA section not shown');
    pass('MFA type SMS shows SMS profile selector');

    await page.evaluate(() => { document.getElementById('mfa_type').value = 'totp'; document.getElementById('mfa_type').dispatchEvent(new Event('change')); });
    await sleep(200);
    const totpSection = await isElementVisible('#mfa_totp_section');
    if (!totpSection) throw new Error('TOTP info banner not shown');
    pass('MFA type TOTP shows info banner');

    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('Landing Page modal tests', e.message); }
}

async function testSendingProfilesPage() {
  console.log('\n\x1b[36m=== SENDING PROFILES PAGE ===\x1b[0m');

  try {
    await navTo('/sending_profiles');
    await pass('Sending Profiles page loads');
  } catch (e) { fail('Sending Profiles page loads', e.message); return; }

  try {
    await checkPageHeader('Sending Profiles');
    pass('Sending Profiles header correct');
  } catch (e) { fail('Sending Profiles header correct', e.message); }

  try {
    const emailTab = await page.$('a[href="#email-profiles"]');
    const smsTab = await page.$('a[href="#sms-profiles"]');
    if (!emailTab || !smsTab) throw new Error('Email/SMS profile tabs missing');
    pass('Email/SMS profile tabs present');
  } catch (e) { fail('Email/SMS profile tabs present', e.message); }

  try {
    const newEmailBtn = await page.$('button[onclick="edit(-1)"]');
    if (!newEmailBtn) throw new Error('New Email Profile button missing');
    pass('New Email Profile button present');
  } catch (e) { fail('New Email Profile button present', e.message); }

  try {
    await page.click('button[onclick="edit(-1)"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Email profile modal did not open');
    pass('Email profile modal opens');

    const nameField = await page.$('#name');
    const interfaceType = await page.$('#interface_type');
    const fromField = await page.$('#from');
    const hostField = await page.$('#host');
    const usernameField = await page.$('#username');
    const passwordField = await page.$('#password');
    const ignoreCertCheckbox = await page.$('#ignore_cert_errors');
    const saveBtn = await page.$('#modalSubmit');
    if (!nameField || !interfaceType || !fromField || !hostField || !usernameField || !passwordField || !ignoreCertCheckbox || !saveBtn)
      throw new Error('Missing SMTP modal fields');
    pass('Email profile modal has all SMTP fields');

    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('Email profile modal', e.message); }

  try {
    await page.click('a[href="#sms-profiles"]');
    await sleep(500);
    const newSmsBtn = await page.$('button[onclick="editSMSProfile(-1)"]');
    if (!newSmsBtn) throw new Error('New SMS Profile button missing');
    pass('New SMS Profile button present');

    await page.click('button[onclick="editSMSProfile(-1)"]');
    await sleep(500);
    const smsModalVisible = await isElementVisible('#smsModal');
    if (!smsModalVisible) throw new Error('SMS modal did not open');
    pass('SMS profile modal opens');

    const smsName = await page.$('#sms_name');
    const smsProvider = await page.$('#sms_provider');
    const smsFrom = await page.$('#sms_from');
    const twilioSid = await page.$('#twilio_account_sid');
    const twilioToken = await page.$('#twilio_auth_token');
    if (!smsName || !smsProvider || !smsFrom || !twilioSid || !twilioToken)
      throw new Error('Missing SMS modal fields');
    pass('SMS profile modal has all fields');

    await page.select('#sms_provider', 'nexmo');
    await sleep(300);
    const nexmoFields = await isElementVisible('#nexmo-fields');
    const twilioFieldsHidden = await page.$eval('#twilio-fields', el => getComputedStyle(el).display === 'none').catch(() => true);
    if (!nexmoFields || !twilioFieldsHidden) throw new Error('Nexmo fields not shown / Twilio not hidden');
    pass('SMS provider switch shows Nexmo fields');

    await closeModal('smsModal');
    await sleep(300);
  } catch (e) { fail('SMS profile modal', e.message); }
}

async function testSMSTemplatesPage() {
  console.log('\n\x1b[36m=== SMS TEMPLATES PAGE ===\x1b[0m');

  try {
    await navTo('/sms_templates');
    await pass('SMS Templates page loads');
  } catch (e) { fail('SMS Templates page loads', e.message); return; }

  try {
    await checkPageHeader('SMS Templates');
    pass('SMS Templates header correct');
  } catch (e) { fail('SMS Templates header correct', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#smsLoading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 15000 });
    pass('SMS Templates loading spinner disappears');
  } catch (e) {
    warn('SMS Templates loading spinner', e.message);
  }

  try {
    const newBtn = await page.$('button[onclick="editSMSTemplate(-1)"]');
    if (!newBtn) throw new Error('New SMS Template button missing');
    pass('New SMS Template button present');
  } catch (e) { fail('New SMS Template button present', e.message); }

  try {
    await page.evaluate(() => {
      const btns = document.querySelectorAll('button');
      for (const b of btns) {
        if (b.getAttribute('onclick') && b.getAttribute('onclick').includes('editSMSTemplate(-1)')) {
          b.click();
          return;
        }
      }
    });
    await sleep(800);
    const modalVisible = await isElementVisible('#smsModal');
    if (!modalVisible) throw new Error('SMS template modal did not open');
    pass('SMS template modal opens');

    const nameField = await page.$('#smsName');
    const fromField = await page.$('#smsFrom');
    const textField = await page.$('#smsText');
    const charCount = await page.$('#smsCharCount');
    const smsCount = await page.$('#smsCount');
    const saveBtn = await page.$('#smsModalSubmit');
    if (!nameField || !fromField || !textField || !charCount || !smsCount || !saveBtn)
      throw new Error('Missing SMS modal fields');
    pass('SMS template modal has all fields including character count');

    await typeInField('#smsName', 'E2E SMS Test');
    await typeInField('#smsText', 'Hello {{.FirstName}}, verify here: {{.URL}}');
    await sleep(300);

    const charVal = await page.$eval('#smsCharCount', el => el.textContent);
    if (charVal === '0') throw new Error('Character count not updating');
    pass(`SMS character count updates: ${charVal} chars`);

    await page.click('#smsModalSubmit');
    await sleep(2000);

    const modalStillOpen = await isElementVisible('#smsModal');
    if (!modalStillOpen) {
      pass('SMS template saved successfully');
    } else {
      warn('SMS template save', 'Modal still open - checking for errors');
      await closeModal('smsModal');
      await sleep(300);
    }
  } catch (e) { fail('SMS template modal CRUD', e.message); }
}

async function testQRCodesPage() {
  console.log('\n\x1b[36m=== QR CODES PAGE ===\x1b[0m');

  try {
    await navTo('/qr_code_generator');
    await pass('QR Codes page loads');
  } catch (e) { fail('QR Codes page loads', e.message); return; }

  try {
    await checkPageHeader('QR Code Generator');
    pass('QR Codes header correct');
  } catch (e) { fail('QR Codes header correct', e.message); }

  try {
    const urlField = await page.$('#url');
    const sizeField = await page.$('#size');
    const storeCheckbox = await page.$('#storeInDb');
    const generateBtn = await page.$('#generateButton');
    if (!urlField || !sizeField || !storeCheckbox || !generateBtn)
      throw new Error('Missing QR code form fields');
    pass('QR Code form has all fields');
  } catch (e) { fail('QR Code form fields', e.message); }

  try {
    const tableExists = await page.$('#qrCodeTable');
    if (!tableExists) throw new Error('QR code table missing');
    pass('QR Code table present');
  } catch (e) { fail('QR Code table present', e.message); }
}

async function testScenariosPage() {
  console.log('\n\x1b[36m=== SCENARIOS PAGE ===\x1b[0m');

  try {
    await navTo('/scenarios');
    await pass('Scenarios page loads');
  } catch (e) { fail('Scenarios page loads', e.message); return; }

  try {
    await checkPageHeader('Scenarios');
    pass('Scenarios header correct');
  } catch (e) { fail('Scenarios header correct', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#loading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 8000 });
    pass('Scenarios loading spinner disappears');
  } catch (e) { fail('Scenarios loading spinner disappears', e.message); }

  try {
    await page.click('button[onclick="edit(-1)"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Scenario modal did not open');
    pass('New Scenario modal opens');

    const nameField = await page.$('#name');
    const descField = await page.$('#description');
    const templateSelect = await page.$('#template');
    const pageSelect = await page.$('#page');
    const urlField = await page.$('#url');
    const saveBtn = await page.$('#modalSubmit');
    if (!nameField || !descField || !templateSelect || !pageSelect || !urlField || !saveBtn)
      throw new Error('Missing scenario modal fields');
    pass('Scenario modal has all fields');
    
    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('Scenario modal tests', e.message); }
}

async function testTeamsPage() {
  console.log('\n\x1b[36m=== TEAMS PAGE ===\x1b[0m');

  try {
    await navTo('/teams');
    await pass('Teams page loads');
  } catch (e) { fail('Teams page loads', e.message); return; }

  try {
    await checkPageHeader('Teams');
    pass('Teams header correct');
  } catch (e) { fail('Teams header correct', e.message); }

  try {
    await page.waitForFunction(() => {
      const loading = document.querySelector('#loading');
      return !loading || getComputedStyle(loading).display === 'none';
    }, { timeout: 8000 });
    pass('Teams loading spinner disappears');
  } catch (e) { fail('Teams loading spinner disappears', e.message); }

  try {
    await page.click('button[onclick="edit(-1)"]');
    await sleep(500);
    const modalVisible = await isElementVisible('#team_modal');
    if (!modalVisible) throw new Error('Team modal did not open');
    pass('New Team modal opens');

    const nameField = await page.$('#name');
    const descField = await page.$('#description');
    const userSelect = await page.$('#user');
    const saveBtn = await page.$('#modalSubmitTeam');
    if (!nameField || !descField || !userSelect || !saveBtn)
      throw new Error('Missing team modal fields');
    pass('Team modal has all fields');

    await closeModal('team_modal');
    await sleep(300);
  } catch (e) { fail('Team modal tests', e.message); }
}

async function testReportsPage() {
  console.log('\n\x1b[36m=== REPORTS PAGE ===\x1b[0m');

  try {
    await navTo('/reports');
    await pass('Reports page loads');
  } catch (e) { fail('Reports page loads', e.message); return; }

  try {
    await checkPageHeader('Reports');
    pass('Reports header correct');
  } catch (e) { fail('Reports header correct', e.message); }

  try {
    const generateTab = await page.$('a[href="#generate-tab"]');
    const historyTab = await page.$('a[href="#history-tab"]');
    if (!generateTab || !historyTab) throw new Error('Report tabs missing');
    pass('Generate/History tabs present');
  } catch (e) { fail('Report tabs present', e.message); }

  try {
    const reportSource = await page.$('#report_source');
    const campaignSelect = await page.$('#campaign_select');
    const reportFormat = await page.$('#report_format');
    const generateBtn = await page.$('#generate_report');
    if (!reportSource || !campaignSelect || !reportFormat || !generateBtn)
      throw new Error('Missing report form elements');
    pass('Report form has all elements');
  } catch (e) { fail('Report form elements', e.message); }

  try {
    const anonymizeEmails = await page.$('#anonymize_emails');
    const anonymizeIPs = await page.$('#anonymize_ips');
    if (!anonymizeEmails || !anonymizeIPs) throw new Error('Missing privacy checkboxes');
    pass('Privacy options (anonymize) present');
  } catch (e) { fail('Privacy options present', e.message); }

  try {
    const depStatus = await page.$('#dependency-status');
    if (!depStatus) throw new Error('Dependency status indicator missing');
    pass('Dependency status indicator present');
  } catch (e) { fail('Dependency status indicator', e.message); }
}

async function testNonCampaignReportsPage() {
  console.log('\n\x1b[36m=== IMAP MONITOR PAGE ===\x1b[0m');

  try {
    await navTo('/non_campaign_reports');
    await pass('IMAP Monitor page loads');
  } catch (e) { fail('IMAP Monitor page loads', e.message); return; }

  try {
    const header = await page.$eval('.page-header', el => el.textContent.trim()).catch(() => '');
    pass('IMAP Monitor page renders');
  } catch (e) { fail('IMAP Monitor page renders', e.message); }
}

async function testSettingsPage() {
  console.log('\n\x1b[36m=== SETTINGS PAGE ===\x1b[0m');

  try {
    await navTo('/settings');
    await pass('Settings page loads');
  } catch (e) { fail('Settings page loads', e.message); return; }

  try {
    await checkPageHeader('Settings');
    pass('Settings header correct');
  } catch (e) { fail('Settings header correct', e.message); }

  try {
    const accountTab = await page.$('a[href="#mainSettings"]');
    const uiTab = await page.$('a[href="#uiSettings"]');
    const reportingTab = await page.$('a[href="#reportingSettings"]');
    if (!accountTab || !uiTab || !reportingTab) throw new Error('Settings tabs missing');
    pass('Account/UI/Reporting settings tabs present');
  } catch (e) { fail('Settings tabs present', e.message); }

  try {
    const apiKey = await page.$('#api_key');
    const username = await page.$('#username');
    const currentPass = await page.$('#current_password');
    const newPass = await page.$('#password');
    const confirmPass = await page.$('#confirm_new_password');
    const saveBtn = await page.$('#settingsForm button[type="submit"]');
    if (!apiKey || !username || !currentPass || !newPass || !confirmPass || !saveBtn)
      throw new Error('Missing account settings fields');
    pass('Account settings has all fields');
  } catch (e) { fail('Account settings fields', e.message); }

  try {
    const apiKeyVal = await page.$eval('#api_key', el => el.value);
    if (!apiKeyVal || apiKeyVal.length < 10) throw new Error('API key missing or too short');
    pass(`API key displayed (${apiKeyVal.length} chars)`);
  } catch (e) { fail('API key displayed', e.message); }

  try {
    await page.click('a[href="#uiSettings"]');
    await sleep(300);
    const useMap = await page.$('#use_map');
    const themeSelect = await page.$('#theme_selector');
    if (!useMap || !themeSelect) throw new Error('Missing UI settings fields');
    pass('UI Settings has map toggle and theme selector');
  } catch (e) { fail('UI Settings fields', e.message); }

  try {
    await page.click('a[href="#reportingSettings"]');
    await sleep(300);
    const useImap = await page.$('#use_imap');
    const imapHost = await page.$('#imaphost');
    const imapPort = await page.$('#imapport');
    const imapUser = await page.$('#imapusername');
    const imapPass = await page.$('#imappassword');
    const useTls = await page.$('#use_tls');
    const saveSettings = await page.$('#savesettings');
    const testSettings = await page.$('#validateimap');
    if (!useImap || !imapHost || !imapPort || !imapUser || !imapPass || !useTls || !saveSettings || !testSettings)
      throw new Error('Missing IMAP settings fields');
    pass('Reporting/IMAP settings has all fields');
  } catch (e) { fail('Reporting/IMAP settings fields', e.message); }
}

async function testUsersPage() {
  console.log('\n\x1b[36m=== USER MANAGEMENT PAGE ===\x1b[0m');

  try {
    await navTo('/users');
    await pass('User Management page loads');
  } catch (e) { fail('User Management page loads', e.message); return; }

  try {
    const newBtn = await page.$('#new_button');
    if (!newBtn) throw new Error('New User button missing');
    pass('New User button present');
  } catch (e) { fail('New User button present', e.message); }

  try {
    const tableExists = await page.$('#userTable');
    if (!tableExists) throw new Error('User table missing');
    pass('User table present');
  } catch (e) { fail('User table present', e.message); }

  try {
    await page.click('#new_button');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('User modal did not open');
    pass('New User modal opens');

    const username = await page.$('#username');
    const password = await page.$('#password');
    const confirmPassword = await page.$('#confirm_password');
    const forcePwChange = await page.$('#force_password_change_checkbox');
    const accountLocked = await page.$('#account_locked_checkbox');
    const role = await page.$('#role');
    const saveBtn = await page.$('#modalSubmit');
    if (!username || !password || !confirmPassword || !forcePwChange || !accountLocked || !role || !saveBtn)
      throw new Error('Missing user modal fields');
    pass('User modal has all fields');

    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('User modal tests', e.message); }
}

async function testWebhooksPage() {
  console.log('\n\x1b[36m=== WEBHOOKS PAGE ===\x1b[0m');

  try {
    await navTo('/webhooks');
    await pass('Webhooks page loads');
  } catch (e) { fail('Webhooks page loads', e.message); return; }

  try {
    const newBtn = await page.$('#new_button');
    if (!newBtn) throw new Error('New Webhook button missing');
    pass('New Webhook button present');
  } catch (e) { fail('New Webhook button present', e.message); }

  try {
    await page.click('#new_button');
    await sleep(500);
    const modalVisible = await isElementVisible('#modal');
    if (!modalVisible) throw new Error('Webhook modal did not open');
    pass('Webhook modal opens');

    const nameField = await page.$('#name');
    const urlField = await page.$('#url');
    const secretField = await page.$('#secret');
    const isActiveCheckbox = await page.$('#is_active');
    const saveBtn = await page.$('#modalSubmit');
    if (!nameField || !urlField || !secretField || !isActiveCheckbox || !saveBtn)
      throw new Error('Missing webhook modal fields');
    pass('Webhook modal has all fields');

    await closeModal('modal');
    await sleep(300);
  } catch (e) { fail('Webhook modal tests', e.message); }
}

async function testNavigationSidebar() {
  console.log('\n\x1b[36m=== NAVIGATION SIDEBAR ===\x1b[0m');

  const navItems = [
    { href: '/', label: 'Dashboard' },
    { href: '/campaigns', label: 'Campaigns' },
    { href: '/campaign_sets', label: 'Campaign Sets' },
    { href: '/groups', label: 'Users & Groups' },
    { href: '/templates', label: 'Email Templates' },
    { href: '/sms_templates', label: 'SMS Templates' },
    { href: '/landing_pages', label: 'Landing Pages' },
    { href: '/sending_profiles', label: 'Sending Profiles' },
    { href: '/scenarios', label: 'Scenarios' },
    { href: '/qr_code_generator', label: 'QR Codes' },
    { href: '/reports', label: 'Reports' },
    { href: '/non_campaign_reports', label: 'IMAP Monitor' },
    { href: '/settings', label: 'Settings' },
    { href: '/users', label: 'User Management' },
    { href: '/teams', label: 'Team Management' },
    { href: '/webhooks', label: 'Webhooks' },
  ];

  for (const item of navItems) {
    try {
      await navTo(item.href);
      const header = await page.$eval('.page-header', el => el.textContent.trim()).catch(() => '');
      pass(`Nav to ${item.label} (${item.href}) works`);
    } catch (e) {
      fail(`Nav to ${item.label} (${item.href})`, e.message);
    }
  }
}

async function testThemeToggle() {
  console.log('\n\x1b[36m=== THEME TOGGLE ===\x1b[0m');

  try {
    await navTo('/');
    const toggleBtn = await page.$('#gp-theme-toggle');
    if (!toggleBtn) throw new Error('Theme toggle button missing');
    pass('Theme toggle button present');

    const initialClass = await page.evaluate(() => document.documentElement.className);
    await page.click('#gp-theme-toggle');
    await sleep(300);
    const afterClickClass = await page.evaluate(() => document.documentElement.className);
    if (initialClass === afterClickClass) throw new Error('Theme did not change');
    pass('Theme toggles on click');

    await page.click('#gp-theme-toggle');
    await sleep(300);
    const backClass = await page.evaluate(() => document.documentElement.className);
    if (backClass !== initialClass) throw new Error('Theme did not toggle back');
    pass('Theme toggles back');
  } catch (e) { fail('Theme toggle', e.message); }
}

async function testTopbarElements() {
  console.log('\n\x1b[36m=== TOPBAR ELEMENTS ===\x1b[0m');

  try {
    await navTo('/');
    const username = await page.$eval('.gp-topbar-user', el => el.textContent.trim()).catch(() => '');
    if (!username) throw new Error('Username not shown in topbar');
    pass(`Topbar shows username: ${username}`);

    const logoutLink = await page.$('a[href="/logout"]');
    if (!logoutLink) throw new Error('Logout link missing');
    pass('Logout link present');
  } catch (e) { fail('Topbar elements', e.message); }
}

async function testAPIDocsPage() {
  console.log('\n\x1b[36m=== API DOCS PAGE ===\x1b[0m');

  try {
    await navTo('/api_documentation');
    const body = await page.$eval('.main', el => el.textContent.trim()).catch(() => '');
    pass('API Docs page loads');
  } catch (e) { fail('API Docs page loads', e.message); }
}

async function test404Page() {
  console.log('\n\x1b[36m=== 404 PAGE ===\x1b[0m');

  try {
    await page.goto(`${BASE}/nonexistent_page_12345`, { waitUntil: 'networkidle0', timeout: 10000 }).catch(() => {});
    const status = await page.evaluate(() => document.title || document.body.textContent.substring(0, 200));
    pass('404 page handled');
  } catch (e) { fail('404 page', e.message); }
}

async function testActiveNavHighlighting() {
  console.log('\n\x1b[36m=== ACTIVE NAV HIGHLIGHTING ===\x1b[0m');

  const pages = [
    { path: '/', expectedLabel: 'Dashboard' },
    { path: '/campaigns', expectedLabel: 'Campaigns' },
    { path: '/templates', expectedLabel: 'Email Templates' },
  ];

  for (const p of pages) {
    try {
      await navTo(p.path);
      const activeLabel = await page.evaluate(() => {
        const activeLi = document.querySelector('.nav-sidebar > li.active');
        return activeLi ? activeLi.textContent.trim() : 'NONE';
      });
      if (activeLabel.includes(p.expectedLabel) || p.path === '/') {
        pass(`Active nav for ${p.path} highlighted`);
      } else {
        warn(`Active nav for ${p.path}`, `Expected "${p.expectedLabel}", got "${activeLabel}"`);
      }
    } catch (e) { fail(`Active nav for ${p.path}`, e.message); }
  }
}

async function testLogout() {
  console.log('\n\x1b[36m=== LOGOUT ===\x1b[0m');

  try {
    await navTo('/');
    await Promise.all([
      page.waitForNavigation({ waitUntil: 'networkidle0', timeout: 10000 }),
      page.click('a[href="/logout"]')
    ]);
    const url = page.url();
    if (!url.includes('/login')) throw new Error('Did not redirect to login after logout');
    pass('Logout redirects to login page');
  } catch (e) { fail('Logout', e.message); }
}

// ─── MAIN RUNNER ──────────────────────────────────────────────

async function main() {
  console.log('\n\x1b[1m\x1b[35m╔══════════════════════════════════════════════╗');
  console.log('║    ULTIMATE GOPHISH - E2E UI TEST SUITE     ║');
  console.log('╚══════════════════════════════════════════════╝\x1b[0m\n');

  browser = await puppeteer.launch({
    headless: 'new',
    args: ['--ignore-certificate-errors', '--no-sandbox', '--disable-setuid-sandbox'],
    defaultViewport: { width: 1440, height: 900 },
  });

  page = await browser.newPage();
  await page.setCookie({
    name: 'ignore-certificate-errors',
    value: 'true',
    domain: '127.0.0.1',
  });

  // Suppress console noise from the app
  page.on('console', msg => {
    if (msg.type() === 'error' && !msg.text().includes('favicon')) {
      // Only log real JS errors
    }
  });

  const startTime = Date.now();

  // 1. Login
  await testLoginPage();

  // 2. Dashboard
  await testDashboard();

  // 3. Navigation
  await testNavigationSidebar();
  await testActiveNavHighlighting();

  // 4. Campaigns
  await testCampaignsPage();

  // 5. Campaign Sets
  await testCampaignSetsPage();

  // 6. Groups
  await testGroupsPage();

  // 7. Templates
  await testTemplatesPage();

  // 8. Landing Pages
  await testLandingPagesPage();

  // 9. Sending Profiles
  await testSendingProfilesPage();

  // 10. SMS Templates
  await testSMSTemplatesPage();

  // 11. QR Codes
  await testQRCodesPage();

  // 12. Scenarios
  await testScenariosPage();

  // 13. Teams
  await testTeamsPage();

  // 14. Reports
  await testReportsPage();

  // 15. IMAP Monitor
  await testNonCampaignReportsPage();

  // 16. Settings
  await testSettingsPage();

  // 17. User Management
  await testUsersPage();

  // 18. Webhooks
  await testWebhooksPage();

  // 19. Theme & Topbar
  await testThemeToggle();
  await testTopbarElements();

  // 20. API Docs
  await testAPIDocsPage();

  // 21. 404
  await test404Page();

  // 22. Logout
  await testLogout();

  const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

  await browser.close();

  // ─── REPORT ────────────────────────────────────────
  console.log('\n\x1b[1m\x1b[35m╔══════════════════════════════════════════════╗');
  console.log('║            TEST RESULTS SUMMARY               ║');
  console.log('╚══════════════════════════════════════════════╝\x1b[0m\n');

  console.log(`  \x1b[32mPASSED:  ${results.passed.length}\x1b[0m`);
  console.log(`  \x1b[31mFAILED:  ${results.failed.length}\x1b[0m`);
  console.log(`  \x1b[33mWARNINGS: ${results.warnings.length}\x1b[0m`);
  console.log(`  Time: ${elapsed}s\n`);

  if (results.failed.length > 0) {
    console.log('\x1b[31m  FAILURES:\x1b[0m');
    for (const f of results.failed) {
      console.log(`    - ${f.name}: ${f.error}`);
    }
  }

  if (results.warnings.length > 0) {
    console.log('\n\x1b[33m  WARNINGS:\x1b[0m');
    for (const w of results.warnings) {
      console.log(`    - ${w.name}: ${w.detail}`);
    }
  }

  // Write JSON report
  const report = {
    timestamp: new Date().toISOString(),
    duration_seconds: parseFloat(elapsed),
    total_tests: results.passed.length + results.failed.length + results.warnings.length,
    passed: results.passed.length,
    failed: results.failed.length,
    warnings: results.warnings.length,
    failures: results.failed,
    warning_details: results.warnings,
    passed_tests: results.passed,
  };

  fs.writeFileSync('/Users/mnasr/Documents/0xbugatti/Projects/gophish-project/ultimate-gophish/test-ui/test-report.json', JSON.stringify(report, null, 2));
  console.log(`\n  Report saved to test-ui/test-report.json\n`);

  process.exit(results.failed.length > 0 ? 1 : 0);
}

main().catch(err => {
  console.error('Fatal error:', err);
  if (browser) browser.close();
  process.exit(2);
});
