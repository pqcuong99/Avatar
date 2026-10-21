import { lazy, Suspense, useCallback, useEffect, useState } from 'react';
import { ApiError, get } from './api/http.ts';
import type { Me } from './api/types.ts';
import { AuthScreen } from './features/auth/AuthScreen.tsx';
import { CreateCharacter } from './features/character/CreateCharacter.tsx';
import { GameView } from './game/GameView.tsx';
import { post } from './api/http.ts';

type Phase = { kind: 'loading' } | { kind: 'auth' } | { kind: 'create'; me: Me } | { kind: 'game'; me: Me } | { kind: 'error'; message: string };

const AdminApp = lazy(() => import('./admin/AdminApp.tsx'));

export function App() {
  if (location.pathname.startsWith('/admin')) {
    return <Suspense fallback={<div className="center-screen"><div className="spinner" /></div>}><AdminApp /></Suspense>;
  }
  return <PlayerApp />;
}

function PlayerApp() {
  const [phase, setPhase] = useState<Phase>({ kind: 'loading' });

  const check = useCallback(async () => {
    try {
      const env = await get<Me>('/api/v1/auth/me');
      setPhase(env.result.character ? { kind: 'game', me: env.result } : { kind: 'create', me: env.result });
    } catch (e) {
      if (e instanceof ApiError && (e.status === 401 || e.status === 403)) setPhase({ kind: 'auth' });
      else setPhase({ kind: 'error', message: 'Không kết nối được máy chủ game.' });
    }
  }, []);

  useEffect(() => { void check(); }, [check]);

  const logout = async () => {
    try { await post('/api/v1/auth/logout'); } catch { /* ignore */ }
    setPhase({ kind: 'auth' });
  };

  switch (phase.kind) {
    case 'loading':
      return <div className="center-screen"><div className="spinner" aria-label="Đang tải" /></div>;
    case 'error':
      return (
        <div className="center-screen">
          <p>{phase.message}</p>
          <button className="btn primary" onClick={() => { setPhase({ kind: 'loading' }); void check(); }}>Thử lại</button>
        </div>
      );
    case 'auth':
      return <AuthScreen onLoggedIn={() => void check()} />;
    case 'create':
      return <CreateCharacter onCreated={() => void check()} onLogout={() => void logout()} />;
    case 'game':
      return <GameView me={phase.me} onLogout={() => setPhase({ kind: 'auth' })} />;
  }
}
