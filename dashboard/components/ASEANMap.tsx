"use client";

import { MapContainer, TileLayer, CircleMarker, Polyline, Tooltip } from "react-leaflet";
import { BankNode, Transaction } from "@/lib/types";
import "leaflet/dist/leaflet.css";

interface ASEANMapProps {
  countries: any[]; // Deprecated
  bankNodes: BankNode[];
  transactions: Transaction[];
  onNodeClick: (node: BankNode) => void;
  selectedNode: BankNode | null;
}

export default function ASEANMap({
  bankNodes,
  transactions,
  onNodeClick,
  selectedNode,
}: ASEANMapProps) {
  // Calculate center of ASEAN roughly
  const center: [number, number] = [6.0, 110.0];
  const zoom = 5;
  
  // Bounds for Southeast Asia to prevent world wrap
  const maxBounds: [[number, number], [number, number]] = [
    [-15, 90], // Southwest
    [25, 145], // Northeast
  ];

  return (
    <div className="relative w-full h-full bg-surface rounded border border-border overflow-hidden">
      {/* Map Title Overlay */}
      <div className="absolute top-4 left-4 z-[400] pointer-events-none">
        <h2 className="text-xs font-bold text-text-muted uppercase tracking-widest flex items-center gap-2 bg-surface/80 p-2 rounded backdrop-blur-sm border border-border">
          <div className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
          Geospatial Network
        </h2>
      </div>

      <MapContainer
        center={center}
        zoom={zoom}
        minZoom={4}
        maxZoom={8}
        maxBounds={maxBounds}
        maxBoundsViscosity={1.0}
        style={{ height: "100%", width: "100%", background: "#0d1117" }}
        zoomControl={false}
        attributionControl={false}
      >
        {/* Dark Matter Tiles */}
        <TileLayer
          url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
        />

        {/* Connection Lines */}
        {bankNodes.map((node, i) =>
          bankNodes.slice(i + 1).map((otherNode, j) => {
            const txCount = transactions.filter(
              (tx) =>
                (tx.sourceCountry === node.countryId &&
                  tx.destCountry === otherNode.countryId) ||
                (tx.sourceCountry === otherNode.countryId &&
                  tx.destCountry === node.countryId)
            ).length;

            if (txCount === 0) return null;

            return (
              <Polyline
                key={`${node.id}-${otherNode.id}`}
                positions={[
                  [node.lat, node.lng],
                  [otherNode.lat, otherNode.lng],
                ]}
                pathOptions={{
                  color: "#30363d",
                  weight: 1,
                  opacity: 0.5,
                  dashArray: "5, 10",
                }}
              />
            );
          })
        )}

        {/* Bank Nodes */}
        {bankNodes.map((node) => {
          const recentTxs = transactions.filter(
            (tx) =>
              tx.sourceCountry === node.countryId ||
              tx.destCountry === node.countryId
          ).length;

          const isSelected = selectedNode?.id === node.id;
          const isActive = recentTxs > 0;
          
          // Dynamic Radius
          const baseRadius = 4;
          const dynamicRadius = Math.min(baseRadius + (node.transactionCount * 0.5), 15);

          return (
            <CircleMarker
              key={node.id}
              center={[node.lat, node.lng]}
              radius={dynamicRadius}
              pathOptions={{
                fillColor: isSelected ? "#58a6ff" : isActive ? "#c9d1d9" : "#30363d",
                color: isSelected ? "#58a6ff" : isActive ? "#c9d1d9" : "#30363d",
                weight: 1,
                opacity: 1,
                fillOpacity: isSelected ? 0.8 : 0.6,
              }}
              eventHandlers={{
                click: () => onNodeClick(node),
              }}
            >
              <Tooltip direction="top" offset={[0, -10]} opacity={1} className="custom-tooltip">
                <span className="font-bold text-xs">{node.bic}</span>
              </Tooltip>
            </CircleMarker>
          );
        })}
      </MapContainer>

      {/* Stats Overlay */}
      <div className="absolute bottom-4 right-4 z-[400] bg-background/80 border border-border rounded p-2 backdrop-blur-sm pointer-events-none">
        <div className="text-[10px] text-text-muted font-mono uppercase">
          Nodes: {bankNodes.length} Active | Layer: OSM/CartoDB
        </div>
      </div>
    </div>
  );
}
