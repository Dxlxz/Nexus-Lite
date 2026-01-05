# Nexus-Lite Command Center Dashboard

A sci-fi themed real-time visualization dashboard for cross-border settlement transactions.

## Features

- **ASEAN Map Visualization**: Interactive SVG map showing bank nodes across ASEAN countries
- **Real-Time Transaction Animation**: Animated particles showing transaction flow between countries
- **ISO 20022 XML Detail View**: Syntax-highlighted XML display with transaction metadata
- **Live Statistics**: Real-time metrics including throughput, success rate, and bank balances
- **WebSocket Integration**: Live updates from the Go consumer backend

## Technology Stack

- **Next.js 14**: React framework with App Router
- **TypeScript**: Type-safe development
- **Tailwind CSS**: Utility-first styling
- **Framer Motion**: Smooth animations and transitions
- **Lucide React**: Beautiful icons

## Getting Started

### Prerequisites

- Node.js 18+ installed
- Go consumer with WebSocket server running on port 8080

### Installation

```bash
cd dashboard
npm install
```

### Development

```bash
npm run dev
```

The dashboard will be available at [http://localhost:3000](http://localhost:3000)

### Build

```bash
npm run build
npm start
```

## Environment Variables

Create a `.env.local` file in the dashboard directory:

```env
NEXT_PUBLIC_WS_URL=ws://localhost:8080/ws
```

## Components

### ASEANMap
Interactive SVG map with:
- Bank nodes positioned on ASEAN countries
- Connection lines between active nodes
- Transaction count badges
- Click-to-select functionality

### StatsPanel
Real-time statistics display:
- Total transactions processed
- Messages per second
- Success rate percentage
- Active connections
- Top 5 bank balances

### TransactionList
Recent transactions table with:
- Status indicators (approved/rejected/pending)
- Transaction details (amount, currency, latency)
- Click-to-view XML functionality
- Auto-refresh

### XMLDetailPanel
Slide-out panel showing:
- Transaction metadata
- Syntax-highlighted ISO 20022 XML
- Error codes for rejected transactions
- Copy to clipboard button

## Visual Design

### Color Palette
- Background: Deep space blue/black (#0a0e17)
- Primary: Cyan neon (#00f0ff)
- Success: Green neon (#00ff88)
- Error: Red neon (#ff0055)
- Warning: Yellow neon (#ffcc00)
- Text: White/off-white (#e0e0e0)

### Effects
- Glow effects on active elements
- Scanline overlay
- Animated borders
- Pulse animations
- Grid background pattern

## WebSocket API

### Connection

Connect to WebSocket server:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

### Message Types

#### Transaction
```json
{
  "type": "transaction",
  "data": {
    "id": "tx-1234567890",
    "msgId": "PAYNET-NEXUS-1704201800-0001",
    "source": "Maybank",
    "sourceCountry": "MY",
    "destination": "DBS Bank",
    "destCountry": "SG",
    "amount": 15000,
    "currency": "MYR",
    "status": "approved",
    "timestamp": "2024-01-02T14:50:00Z",
    "xml": "...",
    "latency": 45
  }
}
```

#### Metrics
```json
{
  "type": "metrics",
  "data": {
    "totalProcessed": 1000,
    "messagesPerSecond": 50.5,
    "successRate": 98.5,
    "activeConnections": 6
  }
}
```

#### Status
```json
{
  "type": "status",
  "data": {
    "status": "connected",
    "message": "Connected to Nexus-Lite WebSocket"
  }
}
```

## Project Structure

```
dashboard/
├── app/
│   ├── layout.tsx          # Root layout with fonts
│   ├── page.tsx            # Main dashboard page
│   └── globals.css         # Global styles and Tailwind
├── components/
│   ├── ASEANMap.tsx       # Interactive map component
│   ├── StatsPanel.tsx      # Statistics display
│   ├── TransactionList.tsx  # Recent transactions
│   ├── TransactionAnimation.tsx  # Transaction animations
│   └── XMLDetailPanel.tsx  # XML detail slide-out
├── lib/
│   ├── types.ts            # TypeScript type definitions
│   └── websocket.ts        # WebSocket client
├── package.json
├── tsconfig.json
├── tailwind.config.ts
├── next.config.js
└── postcss.config.js
```

## Browser Support

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

## License

PayNet Internal Project - Next50 George Town Accord Initiative
