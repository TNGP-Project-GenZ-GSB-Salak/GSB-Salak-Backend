const { DEMO_USERNAME, DEMO_PASSWORD } = require("./fixtures");

async function loginAsDemo(page) {
  await page.goto("/login.html");
  await page.getByTestId("username-input").fill(DEMO_USERNAME);
  await page.getByTestId("password-input").fill(DEMO_PASSWORD);
  await page.getByTestId("submit-button").click();
  await page.waitForURL("**/dashboard.html");
}

module.exports = { loginAsDemo };
