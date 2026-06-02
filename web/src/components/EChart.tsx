import { useEffect, useRef } from "react";
import * as echarts from "echarts";

export function EChart({ option, height = 320 }: { option: echarts.EChartsOption | Record<string, unknown>; height?: number }) {
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!ref.current) {
      return;
    }
    const chart = echarts.init(ref.current);
    chart.setOption(option);
    const onResize = () => chart.resize();
    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
      chart.dispose();
    };
  }, [option]);

  return <div ref={ref} style={{ height }} />;
}
