import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { orgApi } from "../api/org";

export function AcceptInvitePage() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const [status, setStatus] = useState<"pending" | "success" | "error">(
    "pending",
  );
  const [errorMsg, setErrorMsg] = useState("");

  useEffect(() => {
    if (!token) return;
    orgApi
      .acceptInvitation(token)
      .then(() => {
        setStatus("success");
        setTimeout(() => navigate("/login"), 2000);
      })
      .catch((err: unknown) => {
        const msg =
          (err as { response?: { data?: { error?: string } } })?.response?.data
            ?.error ?? "Invalid or expired invitation.";
        setErrorMsg(msg);
        setStatus("error");
      });
  }, [token, navigate]);

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <div className="w-full max-w-sm text-center">
        {status === "pending" && (
          <p className="text-slate-400">Accepting invitation…</p>
        )}
        {status === "success" && (
          <p className="text-brand-300">
            Invitation accepted! Redirecting to sign in…
          </p>
        )}
        {status === "error" && <p className="text-red-400">{errorMsg}</p>}
      </div>
    </div>
  );
}
