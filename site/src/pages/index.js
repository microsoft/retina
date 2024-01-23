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
                <div className={styles.paragraph1}>Retina is a cloud- and vendor-agnostic Kubernetes Network Observability platform which helps customers with enterprise grade DevOps, SecOps and compliance use cases.</div>
                <div>It is designed to cater to cluster network administrators, cluster security administrators and DevOps engineers by providing a centralized platform for monitoring application and network health and security. Retina is capable of collecting telemetry, exporting it to multiple destinations (such as Prometheus, Azure Monitor, and other vendors), and visualizing the data in a variety of ways (like Grafana, Azure Monitor, Azure Log Analytics, etc.).</div>
                </div>
            </div>
        <img className={styles.features} src={featuresImage} alt={"Retina features"}/>
      </main>
    </Layout>
  );
}
