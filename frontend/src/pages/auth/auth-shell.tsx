import type { ReactNode } from 'react';
import { FileText, Quote } from 'lucide-react';

export function AuthShell({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <div className="flex min-h-screen w-full bg-background">
      {/* Branded panel */}
      <div className="relative hidden w-1/2 flex-col justify-between overflow-hidden bg-zinc-950 p-12 text-white lg:flex">
        <div className="flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-md bg-white text-zinc-950">
            <FileText className="size-4.5" />
          </div>
          <span className="text-lg font-semibold tracking-tight">Marginalia</span>
        </div>

        <div className="relative z-10 max-w-md">
          <Quote className="mb-5 size-8 text-white/40" />
          <p className="text-2xl font-medium leading-relaxed">
            Your highlights, notes, and reading — self-hosted and yours alone.
          </p>
          <p className="mt-4 text-sm text-white/60">
            A private home for everything you read and the margins you leave behind.
          </p>
        </div>

        <div className="text-xs text-white/40">
          Self-hosted reading companion
        </div>

        <div className="pointer-events-none absolute -right-24 -top-24 size-96 rounded-full bg-white/5 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-32 -left-16 size-96 rounded-full bg-white/5 blur-3xl" />
      </div>

      {/* Form panel */}
      <div className="flex w-full flex-col items-center justify-center p-6 lg:w-1/2">
        <div className="w-full max-w-sm">
          <div className="mb-8 flex items-center gap-2.5 lg:hidden">
            <div className="flex size-8 items-center justify-center rounded-md bg-primary text-primary-foreground">
              <FileText className="size-4.5" />
            </div>
            <span className="text-lg font-semibold tracking-tight">Marginalia</span>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            {title}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{subtitle}</p>
          <div className="mt-8">{children}</div>
        </div>
      </div>
    </div>
  );
}
