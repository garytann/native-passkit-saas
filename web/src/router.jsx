import React from "react";
import App from "./App.jsx";
import MembersRegisterPage from "./pages/MembersRegister.jsx";

const CUSTOMER_REGISTRATION_PATH = "/members/register";

export default function AppRouter() {
  if (isPath(window.location.pathname, CUSTOMER_REGISTRATION_PATH)) {
    return <MembersRegisterPage />;
  }

  return <App />;
}

function isPath(pathname, targetPath) {
  return pathname.replace(/\/$/, "") === targetPath;
}
