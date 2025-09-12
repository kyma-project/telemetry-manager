---
title: Support for FilterProcessor without OTTL Context Inference
status: Accepted
date: 2025-09-11
---

# 16b. Support for FilterProcessor without OTTL Context Inference

## Context

The Telemetry Transform and Filter API is designed with the assumption that all Transform and FilterProcessor components will support OTTL context inference. However, it turns out the current FilterProcessor in its alpha state does not support OTTL context inference.

We need to decide how to handle this situation to continue delivering Transform and Filter capabilities to our users.

## Proposals

- **Wait for OTTL Context Inference Support:**

  We could wait until the FilterProcessor supports OTTL context inference. However, this approach would delay the availability of Filter capabilities to our users.

  Discussions with the code owners of the FilterProcessor indicate that support for OTTL context inference is not a high priority for them, and it may take a significant amount of time before this feature is implemented.

  The benefit of this approach is that we would not need to implement and maintain a custom solution ourselves, and we would be able to leverage the official FilterProcessor implementation, which will be similar to the TransformProcessor implementation.

- **Implement API to Support Current FilterProcessor:**

  We could implement an API that allows us to support the current FilterProcessor without OTTL context inference. This would enable us to provide Filter capabilities to our users immediately.

  However, this approach would require us to maintain a solution that is not aligned with the official OTTL context inference implemented by TransformProcessor. We would need to ensure that our implementation remains compatible with future updates to the FilterProcessor.

  Migrating existing pipelines could be challenging, as users would need to manually update their configurations to use the new API.

- **Filter API Implementation Using Lower Context:**

  We could implement a Filter API that always uses a lower context level, such as `datapoint`, `spanevent`, or `log`. This would allow us to provide Filter capabilities to our users immediately, without waiting for the official FilterProcessor to support OTTL context inference.

  However, this approach would limit the flexibility of our Filter implementation, as users would not be able to use higher context levels such as `resource` or `scope` directly.

  Migrating existing pipelines could be challenging, as users would need to manually update their configurations to use the new API.