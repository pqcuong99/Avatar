import React from 'react';
import { createRoot } from 'react-dom/client';
import GameCanvas from './GameCanvas';
import './styles.css';

function App() {
  return (
    <main className="layout">
      <header>
        <h1>Project Thanh <span>· Monorepo Starter</span></h1>
        <p>Phaser + React + TypeScript | Go Game Server & Transaction Service</p>
      </header>
      <section className="panel">
        <GameCanvas />
        <p className="help">Di chuyển thử bằng <strong>WASD</strong>, phím mũi tên hoặc nhấp/chạm bản đồ.</p>
        <p className="note">Bản đồ demo chỉ chạy trên Client. Chưa có tài khoản, WebSocket, nông trại hay giao dịch thật.</p>
      </section>
    </main>
  );
}

const mount = document.getElementById('root');
if (!mount) throw new Error('React root not found');
createRoot(mount).render(<React.StrictMode><App /></React.StrictMode>);
