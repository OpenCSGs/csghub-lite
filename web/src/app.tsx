import { LocationProvider, Router, Route } from "preact-iso";
import { Layout } from "./components/Layout";
import { Dashboard } from "./pages/Dashboard";
import { Marketplace } from "./pages/Marketplace";
import { Library } from "./pages/Library";
import { Datasets } from "./pages/Datasets";
import { Chat } from "./pages/Chat";
import { Settings } from "./pages/Settings";
import { AIApps } from "./pages/AIApps";

export function App() {
  return (
    <LocationProvider>
      <Layout>
        <Router>
          <Route path="/" component={Dashboard} />
          <Route path="/marketplace" component={Marketplace} />
          <Route path="/library" component={Library} />
          <Route path="/datasets" component={Datasets} />
          <Route path="/chat" component={Chat} />
          <Route path="/ai-apps" component={AIApps} />
          <Route path="/settings" component={Settings} />
        </Router>
      </Layout>
    </LocationProvider>
  );
}
