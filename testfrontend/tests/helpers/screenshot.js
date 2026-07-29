const fs = require("fs");
const path = require("path");

// Screenshots land in testfrontend/screenshots/<flow>/<case>/NN-label.png,
// numbered in the order they're taken within a test case.
function createShooter(flow, caseName) {
  const dir = path.join(__dirname, "..", "..", "screenshots", flow, caseName);
  fs.mkdirSync(dir, { recursive: true });
  let counter = 0;

  return async function shoot(page, label) {
    counter += 1;
    const filename = `${String(counter).padStart(2, "0")}-${label}.png`;
    await page.screenshot({ path: path.join(dir, filename), fullPage: true });
  };
}

module.exports = { createShooter };
