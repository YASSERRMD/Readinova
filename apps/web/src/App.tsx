import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "./contexts/AuthContext";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { AppShell } from "./components/AppShell";
import { LoginPage } from "./pages/LoginPage";
import { SignupPage } from "./pages/SignupPage";
import { AssessmentsPage } from "./pages/AssessmentsPage";
import { QuestionnairePage } from "./pages/QuestionnairePage";
import { DashboardPage } from "./pages/DashboardPage";
import { TeamPage } from "./pages/TeamPage";
import { AcceptInvitePage } from "./pages/AcceptInvitePage";
import ConnectorsPage from "./pages/ConnectorsPage";
import RecommendationsPage from "./pages/RecommendationsPage";

const queryClient = new QueryClient();

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <Routes>
            <Route
              path="/"
              element={<Navigate to="/app/assessments" replace />}
            />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/signup" element={<SignupPage />} />
            <Route path="/accept/:token" element={<AcceptInvitePage />} />

            <Route element={<ProtectedRoute />}>
              <Route element={<AppShell />}>
                <Route
                  path="/app"
                  element={<Navigate to="/app/assessments" replace />}
                />
                <Route path="/app/assessments" element={<AssessmentsPage />} />
                <Route
                  path="/app/assessments/:id/questionnaire"
                  element={<QuestionnairePage />}
                />
                <Route
                  path="/app/assessments/:id/dashboard"
                  element={<DashboardPage />}
                />
                <Route path="/app/team" element={<TeamPage />} />
                <Route path="/app/connectors" element={<ConnectorsPage />} />
                <Route
                  path="/app/assessments/:id/recommendations"
                  element={<RecommendationsPage />}
                />
              </Route>
            </Route>
          </Routes>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
