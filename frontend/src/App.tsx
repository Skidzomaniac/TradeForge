import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AppLayout } from '@/components/Layout/AppLayout';
import { LeaderboardPage } from '@/pages/Leaderboard/LeaderboardPage';
import { SubmitPage } from '@/pages/Submit/SubmitPage';
import { ResultsPage } from '@/pages/Results/ResultsPage';
import { ProgressPage } from '@/pages/Progress/ProgressPage';
import { LoginPage } from '@/pages/Auth/LoginPage';
import { AdminPage } from '@/pages/Admin/AdminPage';
import { isAuthenticated } from '@/lib/auth';

function Protected({ children }: { children: React.ReactNode }) {
  return isAuthenticated() ? <>{children}</> : <Navigate to="/login" replace />;
}

export default function App() {
  return (
    <BrowserRouter>
      <AppLayout>
        <Routes>
          <Route path="/" element={<Navigate to="/leaderboard" replace />} />
          <Route path="/leaderboard" element={<LeaderboardPage />} />
          <Route path="/progress" element={<ProgressPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/admin" element={<AdminPage />} />
          <Route path="/submit" element={<Protected><SubmitPage /></Protected>} />
          <Route path="/results/:testId" element={<Protected><ResultsPage /></Protected>} />
        </Routes>
      </AppLayout>
    </BrowserRouter>
  );
}
