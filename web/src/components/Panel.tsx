import { PropsWithChildren } from "react";

export function Panel({ children, title, subtitle }: PropsWithChildren<{ title: string; subtitle?: string }>) {
  return (
    <section className="panel">
      <div className="panel-head">
        <div>
          <h3>{title}</h3>
          {subtitle ? <p className="muted">{subtitle}</p> : null}
        </div>
      </div>
      {children}
    </section>
  );
}
