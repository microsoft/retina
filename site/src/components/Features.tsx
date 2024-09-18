import style from "./Features.module.css";
import Gadgets from "./eBPF.svg";
import KubernetesAndContainerAware from "./kubernetes.svg";
import CNI from "./cni.svg";
import Prometheus from "./prometheus.svg";
import HubbleLight from "./hubble-light.svg";
import PacketCapture from "./wireshark.svg";
import clsx from "clsx";
import React from "react";

export function Features() {
  return (
    <section className={style.section}>
      <div className={style.container}>
        <div className={clsx(style.features)}>
          <div className={style.feature}>
            <Gadgets />
            <h3>eBPF Based</h3>
            <p className={style.featureDescription}>
              Leverages eBPF technologies to collect and provide insights into your Kubernetes cluster with minimal overhead
            </p>
          </div>
          <div className={style.feature}>
            <KubernetesAndContainerAware />
            <h3>Platform Agnostic</h3>
            <p className={style.featureDescription}>
              Works with any Cloud or On-Prem Kubernetes distribution and supports multiple OS such as Linux, Windows, Azure Linux, etc
            </p>
          </div>
          <div className={style.feature}>
            <CNI />
            <h3>CNI Agnostic</h3>
            <p className={style.featureDescription}>
              Works with any Container Networking Interfaces (CNIs) like Azure CNI, AWS VPC, etc
            </p>
          </div>
        </div>
        <div className={clsx(style.features)}>
          <div className={style.feature}>
            <Prometheus />
            <h3>Metrics</h3>
            <p className={style.featureDescription}>
              Provides industry standard Prometheus metrics
            </p>
          </div>
          <div className={style.feature}>
            <HubbleLight />
            <h3>Hubble Integration</h3>
            <p className={style.featureDescription}>
              Integrates with Cilium's Hubble for additional network insights such as flows logs, DNS, etc
            </p>
          </div>
          <div className={style.feature}>
            <PacketCapture />
            <h3>Packet Capture</h3>
            <p className={style.featureDescription}>
              Distributed packet captures for deep dive troubleshooting
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
