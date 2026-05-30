import { Navigate, Route, Routes } from 'react-router-dom';
import { AppLayout } from '@/components/app-layout';
import { RequireAuth } from '@/auth/require-auth';
import { LoginPage } from '@/pages/auth/login';
import { RegisterPage } from '@/pages/auth/register';
import { LibraryPage } from '@/pages/library/library';
import { DocumentPage } from '@/pages/document/document';
import { ReviewPage } from '@/pages/review/review';
import { TemplatesPage } from '@/pages/templates/templates';
import { SettingsPage } from '@/pages/settings/settings';

export function AppRoutingSetup() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      <Route
        element={
          <RequireAuth>
            <AppLayout />
          </RequireAuth>
        }
      >
        <Route path="/" element={<LibraryPage />} />
        <Route path="/documents/:id" element={<DocumentPage />} />
        <Route path="/review" element={<ReviewPage />} />
        <Route path="/templates" element={<TemplatesPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
