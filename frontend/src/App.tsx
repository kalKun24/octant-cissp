import { QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router';

import { AppLayout } from '@/components/AppLayout';
import { ThemeProvider } from '@/components/ThemeProvider';
import { createQueryClient } from '@/lib/queryClient';
import { HomePage } from '@/pages/HomePage';
import { NotFoundPage } from '@/pages/NotFoundPage';

export function App() {
  // QueryClient はアプリの生存期間で1つだけ持ちます。
  const [queryClient] = useState(createQueryClient);

  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Routes>
            <Route element={<AppLayout />}>
              <Route index element={<HomePage />} />
              <Route path="*" element={<NotFoundPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
