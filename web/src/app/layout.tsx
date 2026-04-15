import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'gofilen',
  description: 'AI-powered Filen cloud drive manager',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  )
}