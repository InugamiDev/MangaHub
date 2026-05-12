import { Bell, Cable, Globe2, MessageCircle, RadioTower } from "lucide-react";

const protocols = [
  { name: "HTTP", role: "Auth, manga search, library, progress", port: "8080", stage: "Online", icon: Globe2 },
  { name: "TCP", role: "Progress sync broadcasts", port: "9090", stage: "Online", icon: Cable },
  { name: "UDP", role: "Chapter release notifications", port: "9091", stage: "Online", icon: Bell },
  { name: "gRPC", role: "Internal manga and progress service", port: "9092", stage: "Online", icon: RadioTower },
  { name: "WebSocket", role: "Real-time discussion chat", port: "9093", stage: "Online", icon: MessageCircle },
];

export function ProtocolMap() {
  return (
    <div className="card">
      <h3>Five required protocols</h3>
      <p>HTTP handles product flows while TCP, UDP, gRPC, and WebSocket run as dedicated backend services.</p>
      <div className="protocol-row">
        {protocols.map((protocol) => {
          const Icon = protocol.icon;
          return (
            <div className="protocol-item" key={protocol.name}>
              <span className="protocol-name">{protocol.name}</span>
              <span className="muted">
                <Icon size={15} /> {protocol.role} on `{protocol.port}`
              </span>
              <span className="protocol-stage">{protocol.stage}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
