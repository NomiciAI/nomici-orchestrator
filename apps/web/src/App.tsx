import { Background, Controls, ReactFlow } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import './styles.css';

const nodes = [
  {
    id: 'root-agent',
    position: { x: 80, y: 80 },
    data: { label: 'Root Agent' },
    type: 'input',
  },
  {
    id: 'runtime',
    position: { x: 360, y: 80 },
    data: { label: 'Runtime' },
  },
  {
    id: 'trace',
    position: { x: 640, y: 80 },
    data: { label: 'Trace Store' },
    type: 'output',
  },
];

const edges = [
  { id: 'root-runtime', source: 'root-agent', target: 'runtime', label: 'delegates' },
  { id: 'runtime-trace', source: 'runtime', target: 'trace', label: 'emits events' },
];

export function App() {
  return (
    <main className="shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Nomici Console</p>
          <h1>Agent Control Plane</h1>
        </div>
        <span className="status">Gateway scaffold</span>
      </header>
      <section className="canvas" aria-label="Nomici agent graph placeholder">
        <ReactFlow nodes={nodes} edges={edges} fitView>
          <Background />
          <Controls />
        </ReactFlow>
      </section>
    </main>
  );
}
