import { QueryClient } from '@tanstack/react-query';

/**
 * サーバ状態は TanStack Query が保持します（別のグローバルストアは作りません）。
 * Firestore の読み取り課金を抑えるため、既定の staleTime を長めに取ります。
 */
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 60_000,
        gcTime: 5 * 60_000,
        retry: 1,
        refetchOnWindowFocus: false,
      },
    },
  });
}
