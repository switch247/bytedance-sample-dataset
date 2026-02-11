const express = require("express");

const buildApp = () => {
  const app = express();
  app.use(express.json());

  // 1. High-speed telemetry ingestion
  app.post("/telemetry", (req, res) => {
    const { deviceId, sensorValue, timestamp } = req.body;
    if (!deviceId || typeof sensorValue !== "number") {
      return res.status(400).json({ error: "Invalid payload" });
    }
    // Logic: Log ingestion (Simulated)
    res.status(201).json({ status: "recorded", id: deviceId });
  });

  // 2. Parameterized user metadata retriever
  app.get("/user/:id", (req, res) => {
    const userId = req.params.id;
    // Logic: Mock data retrieval
    const userMetadata = {
      id: userId,
      role: "admin",
      lastLogin: Date.now(),
      preferences: { theme: "dark", notifications: true },
    };
    res.json(userMetadata);
  });

  // 3. JSON-heavy data transformer
  app.post("/transform", (req, res) => {
    const { items } = req.body;
    if (!Array.isArray(items)) {
      return res.status(400).json({ error: "Items must be an array" });
    }
    const transformed = items.map((item) => ({
      uid: item.id.toString().toUpperCase(),
      val: item.value * 1.15,
      active: !!item.status,
    }));
    res.json({ processed: transformed, count: transformed.length });
  });

  return app;
};

module.exports = { buildApp };

if (require.main === module) {
  const app = buildApp();
  const PORT = process.env.PORT || 3000;
  app.listen(PORT, () => {
    console.log(`Express gateway running on port ${PORT}`);
  });
}
