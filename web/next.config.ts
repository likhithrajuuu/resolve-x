import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // `next build` and `next dev` must not share a build directory: building while
  // the dev server runs corrupts its route table and makes real pages 404.
  distDir: process.env.NODE_ENV === "development" ? ".next-dev" : ".next",
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
