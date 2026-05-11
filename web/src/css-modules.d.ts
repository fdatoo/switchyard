/// <reference types="vite/client" />

/**
 * TypeScript declaration for CSS Modules.
 * Allows `import styles from './Foo.module.css'` without type errors.
 */
declare module "*.module.css" {
  const classes: Record<string, string>;
  export default classes;
}
