import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from 'next-themes';
import { HelmetProvider } from 'react-helmet-async';
import { BrowserRouter } from 'react-router-dom';
import { LoadingBarContainer } from 'react-top-loading-bar';
import { Toaster } from '@/components/ui/sonner';
import { AuthProvider } from '@/auth/auth-context';
import { AppRouting } from '@/routing/app-routing';

const { BASE_URL } = import.meta.env;

export function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            retry: 1,
            refetchOnWindowFocus: false,
            staleTime: 30_000,
          },
        },
      }),
  );

  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="light"
      storageKey="theme"
      enableSystem
      disableTransitionOnChange
      enableColorScheme
    >
      <HelmetProvider>
        <QueryClientProvider client={queryClient}>
          <LoadingBarContainer>
            <BrowserRouter basename={BASE_URL}>
              <AuthProvider>
                <Toaster />
                <AppRouting />
              </AuthProvider>
            </BrowserRouter>
          </LoadingBarContainer>
        </QueryClientProvider>
      </HelmetProvider>
    </ThemeProvider>
  );
}
