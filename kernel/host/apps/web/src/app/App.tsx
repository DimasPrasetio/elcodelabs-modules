import { AppProvider } from "@/app/providers/AppProvider";
import { RouterProvider } from "@/app/providers/RouterProvider";
import { composition, moduleProviders } from "@/app/composition";
import { CompositionProvider } from "@/shared/composition";

export default function App() {
  const app = moduleProviders.reduceRight((child, Provider) => <Provider>{child}</Provider>, <RouterProvider />);
  return (
    <AppProvider>
      <CompositionProvider value={composition}>{app}</CompositionProvider>
    </AppProvider>
  );
}
