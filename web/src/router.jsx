import React from "react";
import App from "./App.jsx";
import LoginPage from "./pages/Login.jsx";
import MerchantsRegisterPage from "./pages/MerchantsRegister.jsx";
import MembersRegisterPage from "./pages/MembersRegister.jsx";

const CUSTOMER_REGISTRATION_PATH = "/members/register";
const MERCHANT_REGISTRATION_PATH = "/merchants/register";
const DASHBOARD_PATH = "/dashboard";

export default function AppRouter() {
  if (isPath(window.location.pathname, "/") || isPath(window.location.pathname, "/login")) {
    return <LoginPage />;
  }

  if (isPath(window.location.pathname, DASHBOARD_PATH)) {
    return <App />;
  }

  if (isPath(window.location.pathname, MERCHANT_REGISTRATION_PATH)) {
    return <MerchantsRegisterPage />;
  }

  if (isPath(window.location.pathname, CUSTOMER_REGISTRATION_PATH)) {
    return <MembersRegisterPage />;
  }

  return <LoginPage />;
}

function isPath(pathname, targetPath) {
  const normalized = pathname.replace(/\/$/, "") || "/";
  return normalized === targetPath;
}
