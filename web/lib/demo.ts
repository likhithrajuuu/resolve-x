// Sample data for the demo dashboard. Replaced by gateway API calls later.
export type Incident = {
  id: string;
  title: string;
  service: string;
  severity: "SEV-1" | "SEV-2" | "SEV-3";
  status: "Investigating" | "Identified" | "Resolved";
  opened: string;
  cause: string;
  confidence: number;
};

export const incidents: Incident[] = [
  { id: "INC-482", title: "Checkout latency spike", service: "payments", severity: "SEV-2", status: "Identified", opened: "4 min ago", cause: "payments v2.14.0 lowered the DB pool size from 50 to 10", confidence: 87 },
  { id: "INC-481", title: "Elevated 5xx on search", service: "search-api", severity: "SEV-3", status: "Investigating", opened: "22 min ago", cause: "Correlating signals…", confidence: 0 },
  { id: "INC-480", title: "Kafka consumer lag on orders", service: "order-processor", severity: "SEV-2", status: "Resolved", opened: "2 h ago", cause: "Partition rebalance storm after a node drain", confidence: 92 },
  { id: "INC-479", title: "Login failures in eu-west", service: "auth", severity: "SEV-1", status: "Resolved", opened: "yesterday", cause: "Expired TLS certificate on the identity provider", confidence: 96 },
];

export const kpis = [
  { label: "Open incidents", value: "2" },
  { label: "Services discovered", value: "38" },
  { label: "Median time to cause", value: "3m 40s" },
  { label: "Telemetry (24 h)", value: "1.2 B spans" },
];
