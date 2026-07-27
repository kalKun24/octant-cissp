import { QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';
import { BrowserRouter, Route, Routes } from 'react-router';

import { createQueryClient } from '@/lib/queryClient';
import { HomePage } from '@/pages/HomePage';
import { NotFoundPage } from '@/pages/NotFoundPage';

export function App() {
  // QueryClient はアプリの生存期間で1つだけ持ちます。
  const [queryClient] = useState(createQueryClient);

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route index element={<HomePage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
