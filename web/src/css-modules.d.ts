/// <reference types="vite/client" />

// Type declaration for CSS Module imports.
// Vite processes *.module.css files at build time; this shim tells TypeScript
// that each import is a record of class name → scoped string.
declare module "*.module.css" {
  const styles: Record<string, string>;
  export default styles;
}
