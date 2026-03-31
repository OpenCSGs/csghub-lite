declare module "swagger-ui-dist/swagger-ui-bundle.js" {
  interface SwaggerUIBundleOptions {
    url?: string;
    dom_id?: string;
    deepLinking?: boolean;
    displayRequestDuration?: boolean;
    docExpansion?: "none" | "list" | "full";
    filter?: boolean;
    defaultModelsExpandDepth?: number;
    presets?: unknown[];
    layout?: string;
  }

  interface SwaggerUIBundleStatic {
    (options: SwaggerUIBundleOptions): unknown;
    presets: {
      apis: unknown;
    };
  }

  const SwaggerUIBundle: SwaggerUIBundleStatic;
  export default SwaggerUIBundle;
}
