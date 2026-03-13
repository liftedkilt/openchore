import puppeteer from 'puppeteer';

const browser = await puppeteer.launch({
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox'],
  executablePath: '/usr/bin/google-chrome',
});

const page = await browser.newPage();

// Capture all console messages
page.on('console', msg => {
  const type = msg.type();
  if (type === 'error' || type === 'warning') {
    console.log(`[${type}] ${msg.text()}`);
  }
});

// Capture page errors (uncaught exceptions)
page.on('pageerror', err => {
  console.log(`[PAGE ERROR] ${err.message}`);
});

// Capture failed requests
page.on('requestfailed', req => {
  console.log(`[REQUEST FAILED] ${req.url()} - ${req.failure()?.errorText}`);
});

console.log('--- Loading login page ---');
await page.goto('http://localhost:5173/login', { waitUntil: 'networkidle0', timeout: 15000 });

// Check what rendered
const bodyHTML = await page.evaluate(() => document.getElementById('root')?.innerHTML || 'ROOT IS EMPTY');
console.log(`\n--- Root innerHTML (first 500 chars) ---\n${bodyHTML.substring(0, 500)}`);

// Take screenshot
await page.screenshot({ path: '/tmp/login-page.png', fullPage: true });
console.log('\nScreenshot saved to /tmp/login-page.png');

// Now simulate selecting a user and going to dashboard
console.log('\n--- Clicking first user profile ---');
const cards = await page.$$('[class*="profileCard"], [class*="card"], button, a');
console.log(`Found ${cards.length} clickable elements`);

if (cards.length > 0) {
  await cards[0].click();
  await new Promise(r => setTimeout(r, 2000));

  const dashHTML = await page.evaluate(() => document.getElementById('root')?.innerHTML || 'ROOT IS EMPTY');
  console.log(`\n--- Dashboard root innerHTML (first 500 chars) ---\n${dashHTML.substring(0, 500)}`);
  await page.screenshot({ path: '/tmp/dashboard-page.png', fullPage: true });
  console.log('Dashboard screenshot saved to /tmp/dashboard-page.png');
}

await browser.close();
