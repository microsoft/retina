import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import clsx from "clsx";

import styles from "./LandingPage.module.css";
import { Features } from "./Features";
import React from "react";

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx(styles.heroBanner)}>
      <div className={clsx("container", styles.heroContainer)}>
        <Heading as="h1" className={styles.title}>
          Retina
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
      </div>
    </header>
  );
}

export default function LandingPage(): JSX.Element {
  const { siteConfig } = useDocusaurusContext();

  return (
    <Layout wrapperClassName={styles.layoutWrapper}>
      <main>
        <HomepageHeader />
        <Features />
      </main>
    </Layout>
  );
}
