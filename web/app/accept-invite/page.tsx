import { Suspense } from "react";
import { TokenForm } from "@/components/TokenForm";

export const metadata = { title: "Accept invitation — Resolve-X" };
export default function Page() {
  return <Suspense><TokenForm mode="invite" /></Suspense>;
}
