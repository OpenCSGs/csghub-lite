import SwaggerUIBundle from "swagger-ui-dist/swagger-ui-bundle.js";
import "swagger-ui-dist/swagger-ui.css";
import "./api-docs.css";

void SwaggerUIBundle({
  url: "/openapi/local-api.json",
  dom_id: "#swagger-ui",
  deepLinking: true,
  displayRequestDuration: true,
  docExpansion: "list",
  filter: true,
  defaultModelsExpandDepth: -1,
  presets: [SwaggerUIBundle.presets.apis],
  layout: "BaseLayout",
});
