import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'react-hot-toast';
import App from './App';
import { ErrorBoundary } from './components/ErrorBoundary';
import './index.css';

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <App />
        <Toaster position="bottom-right" toastOptions={{ style: { fontFamily: 'monospace', background: '#111', color: '#e8e8e8', border: '1px solid #1a1a1a' } }} />
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
