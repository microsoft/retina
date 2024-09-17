import React from "react";
import styles from "./FeatureHighlight.module.css";
import Gadgets from "./observability-star.svg";

export function FeatureHighlight() {
  return (
    <section className={styles.section}>
      <div className={styles.container}>
        <div className={styles.content}>
          <h2>Observablity tools powered by eBPF</h2>
          <p>
            Inspektor Gadget provides a wide selection of eBPF-based observability Gadgets to dig into Kubernetes and Linux systems
          </p>
        </div>
        <Gadgets className={styles.image} />
      </div>
    </section>
  );
}
