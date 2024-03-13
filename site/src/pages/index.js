import React from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import featuresImage from '@site/static/img/retina-features.png';

import styles from './index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <h1 className="hero__title">{siteConfig.title}</h1>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/intro">
            Get Started 
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={`Hello from ${siteConfig.title}`}
      description="Description will go into a meta tag in <head />">
      <HomepageHeader />
      <main>
            <div className={styles.columns}>
              <div className={styles.column1}>What is Retina?</div>
              <div className={styles.column2}>              
                <div className={styles.paragraph1}>Retina is a cloud-agnostic, open-source Kubernetes Network Observability platform which helps with DevOps, SecOps and compliance use cases. It provides a centralized hub for monitoring application and network health and security, catering to Cluster Network Administrators, Cluster Security Administrators and DevOps Engineers.</div>
                <div>Retina collects customizable telemetry, which can be exported to multiple storage options (such as Prometheus, Azure Monitor, and other vendors) and visualized in a variety of ways (like Grafana, Azure Log Analytics, and other vendors).</div>
                </div>
            </div>
        <img className={styles.features} src={featuresImage} alt={"Retina features"}/>
      </main>
    </Layout>
  );
}
