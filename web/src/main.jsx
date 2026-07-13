import React from "react";
import { createRoot } from "react-dom/client";
import AppRouter from "./router.jsx";
import "./styles.css";

createRoot(document.getElementById("root")).render(<AppRouter />);
