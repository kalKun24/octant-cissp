import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/App';

const container = document.getElementById('root');
if (container === null) {
  throw new Error('マウント先の #root が見つかりません。index.html を確認してください。');
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
