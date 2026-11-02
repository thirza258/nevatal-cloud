import React from 'react';
import ReactDOM from 'react-dom/client';
import { App } from './App';
import { AuthProvider } from './context/AuthContext';
import { SystemConfigProvider } from './context/SystemConfigContext';
import './index.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <SystemConfigProvider>
      <AuthProvider>
        <App />
      </AuthProvider>
    </SystemConfigProvider>
  </React.StrictMode>
);
