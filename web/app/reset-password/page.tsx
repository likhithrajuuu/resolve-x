import { Suspense } from "react";
import { TokenForm } from "@/components/TokenForm";

export const metadata = { title: "Reset password — Resolve-X" };
export default function Page() {
  return <Suspense><TokenForm mode="reset" /></Suspense>;
}
