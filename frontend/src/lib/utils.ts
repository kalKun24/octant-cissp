import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/** Tailwind のクラスを条件付きで結合し、競合するユーティリティを後勝ちで解決します。 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
