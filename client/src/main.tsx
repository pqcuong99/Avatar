import React from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App.tsx';
import './styles.css';

const mount = document.getElementById('root');
if (!mount) throw new Error('React root not found');
createRoot(mount).render(<React.StrictMode><App /></React.StrictMode>);
