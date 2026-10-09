// Two tiny services in one file to demo distributed tracing:
//   RESOLVEX_API_KEY=... node examples/two-services.js
// "storefront" (port 3001) calls "inventory-api" (port 3002) over HTTP.
const { init, logger, withSpan, markDeployment } = require("../dist");

const role = process.argv[2];
const which = role === "inventory" ? "inventory-api" : "storefront";
init({ serviceName: which, version: process.env.APP_VERSION ?? "1.0.0", captureConsole: true });

const http = require("node:http");
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

if (which === "inventory-api") {
  http.createServer(async (req, res) => {
    logger.info("stock lookup start");
    await withSpan("db.query stock", async () => { await sleep(5 + Math.random() * 15); }, { "db.system": "postgresql" });
    if (req.url.includes("fail")) {
      logger.error("stock lookup failed", { sku: "A-1" });
      res.statusCode = 500;
      return res.end("boom");
    }
    res.end(JSON.stringify({ stock: 7 }));
  }).listen(3002, () => console.log("inventory-api on :3002"));
} else {
  http.createServer((req, res) => {
    const target = req.url.includes("fail") ? "/stock?fail=1" : "/stock";
    http.get({ host: "localhost", port: 3002, path: target }, (up) => {
      up.resume();
      up.on("end", () => {
        logger.info("order handled", { status: up.statusCode });
        res.statusCode = up.statusCode === 200 ? 200 : 502;
        res.end("ok");
      });
    }).on("error", () => { res.statusCode = 503; res.end("down"); });
  }).listen(3001, () => console.log("storefront on :3001"));
}
