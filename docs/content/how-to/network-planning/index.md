# Network Planning Guide

Use this interactive tool to plan the network configuration for your Hosted Control Plane.
Select your deployment options step by step and the network diagram will build progressively.

<div id="network-planner"><noscript>This interactive planner requires JavaScript. See <a href="../common/hcp-networking-requirements/">HCP Networking Requirements</a> for a static reference.</noscript></div>

!!! tip "Hostname recommendation for LoadBalancer configurations"
    When using **LoadBalancer** as the publishing strategy for the KAS (Kube API Server), it is strongly recommended to set an explicit **hostname** for all services in your `ServicePublishingStrategyMapping`. This ensures stable DNS resolution and simplifies certificate management. See [Service Publishing Strategies](../../reference/service-publishing-strategies.md) for details.

!!! note "About this tool"
    This planner generates a visual reference for your network topology.
    For detailed port requirements, see [HCP Networking Requirements](../common/hcp-networking-requirements.md).
    For service publishing options, see [Service Publishing Strategies](../../reference/service-publishing-strategies.md).
