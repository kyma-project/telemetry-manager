
### Test in Gardener cluster, with old IsIstioActive function
```
Showing nodes accounting for 90ms, 100% of 90ms total
Showing top 20 nodes out of 58
      flat  flat%   sum%        cum   cum%
      10ms 11.11% 11.11%       10ms 11.11%  runtime.(*mheap).allocSpan
      10ms 11.11% 22.22%       10ms 11.11%  runtime.(*mspan).base (inline)
      10ms 11.11% 33.33%       10ms 11.11%  runtime.(*mspan).writeHeapBitsSmall
      10ms 11.11% 44.44%       10ms 11.11%  runtime.(*spanSet).push
      10ms 11.11% 55.56%       10ms 11.11%  runtime.addb (inline)
      10ms 11.11% 66.67%       40ms 44.44%  runtime.mallocgc
      10ms 11.11% 77.78%       40ms 44.44%  runtime.scanobject
      10ms 11.11% 88.89%       10ms 11.11%  runtime.typePointers.nextFast (inline)
      10ms 11.11%   100%       10ms 11.11%  runtime/internal/atomic.(*Uint32).Add
         0     0%   100%       30ms 33.33%  github.com/kyma-project/telemetry-manager/controllers/telemetry.(*LogPipelineController).Reconcile
         0     0%   100%       30ms 33.33%  github.com/kyma-project/telemetry-manager/internal/istiostatus.(*Checker).IsIstioActive
         0     0%   100%       30ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).Reconcile
         0     0%   100%       30ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).doReconcile
         0     0%   100%       30ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).reconcileFluentBit
         0     0%   100%       10ms 11.11%  golang.org/x/net/http2.(*ClientConn).readLoop
         0     0%   100%       10ms 11.11%  golang.org/x/net/http2.(*clientConnReadLoop).handleResponse
         0     0%   100%       10ms 11.11%  golang.org/x/net/http2.(*clientConnReadLoop).processHeaders
         0     0%   100%       10ms 11.11%  golang.org/x/net/http2.(*clientConnReadLoop).run
         0     0%   100%       30ms 33.33%  k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.(*CustomResourceDefinition).DeepCopy (inline)
         0     0%   100%       30ms 33.33%  k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.(*CustomResourceDefinition).DeepCopyInto
```
<img src="old_pprof_gardener.svg" alt="old_pprof_gardener" width="600" height="1700"/>

### Test in Gardener cluster, with labelselector approach for IsIstioActive
```
Showing nodes accounting for 30ms, 100% of 30ms total
Showing top 30 nodes out of 45
      flat  flat%   sum%        cum   cum%
      10ms 33.33% 33.33%       10ms 33.33%  bytes.(*Buffer).ReadFrom
      10ms 33.33% 66.67%       10ms 33.33%  crypto/internal/bigmod.addMulVVW1024
      10ms 33.33%   100%       10ms 33.33%  io.ReadAll
         0     0%   100%       10ms 33.33%  bufio.(*Reader).Read
         0     0%   100%       10ms 33.33%  crypto/internal/bigmod.(*Nat).Exp
         0     0%   100%       10ms 33.33%  crypto/internal/bigmod.(*Nat).montgomeryMul
         0     0%   100%       10ms 33.33%  crypto/rsa.(*PrivateKey).Sign
         0     0%   100%       10ms 33.33%  crypto/rsa.SignPSS
         0     0%   100%       10ms 33.33%  crypto/rsa.decrypt
         0     0%   100%       10ms 33.33%  crypto/rsa.signPSSWithSalt
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).HandshakeContext
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).Read
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).handshakeContext
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).readFromUntil
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).readRecord (inline)
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).readRecordOrCCS
         0     0%   100%       10ms 33.33%  crypto/tls.(*Conn).serverHandshake
         0     0%   100%       10ms 33.33%  crypto/tls.(*serverHandshakeStateTLS13).handshake
         0     0%   100%       10ms 33.33%  crypto/tls.(*serverHandshakeStateTLS13).sendServerCertificate
         0     0%   100%       10ms 33.33%  github.com/kyma-project/telemetry-manager/controllers/telemetry.(*LogPipelineController).Reconcile
         0     0%   100%       10ms 33.33%  github.com/kyma-project/telemetry-manager/internal/k8sutils.CreateOrUpdateDaemonSet
         0     0%   100%       10ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).Reconcile
         0     0%   100%       10ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).doReconcile
         0     0%   100%       10ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).reconcileFluentBit
         0     0%   100%       10ms 33.33%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).reconcileFluentBit.NewOwnerReferenceSetter.func2
         0     0%   100%       10ms 33.33%  golang.org/x/net/http2.(*ClientConn).readLoop
         0     0%   100%       10ms 33.33%  golang.org/x/net/http2.(*Framer).ReadFrame
         0     0%   100%       10ms 33.33%  golang.org/x/net/http2.(*clientConnReadLoop).run
         0     0%   100%       10ms 33.33%  golang.org/x/net/http2.readFrameHeader
```
<img src="new_pprof_with_labelselector_gardener.svg" alt="new_pprof_with_labelselector_gardener" width="600" height="1700"/>

### Test in Gardener cluster, with discoveryapi approach for IsIstioActive

Showing nodes accounting for 70ms, 100% of 70ms total
Showing top 20 nodes out of 76
flat  flat%   sum%        cum   cum%
20ms 28.57% 28.57%       20ms 28.57%  runtime/internal/syscall.Syscall6
10ms 14.29% 42.86%       10ms 14.29%  encoding/json.stateEndValue
10ms 14.29% 57.14%       10ms 14.29%  k8s.io/client-go/rest.IsValidPathSegmentName
10ms 14.29% 71.43%       30ms 42.86%  runtime.gcDrain
10ms 14.29% 85.71%       10ms 14.29%  runtime.getempty
10ms 14.29%   100%       10ms 14.29%  runtime.scanobject
0     0%   100%       10ms 14.29%  bufio.(*Reader).Read
0     0%   100%       10ms 14.29%  bytes.(*Buffer).ReadFrom
0     0%   100%       10ms 14.29%  crypto/tls.(*Conn).Read
0     0%   100%       10ms 14.29%  crypto/tls.(*Conn).readFromUntil
0     0%   100%       10ms 14.29%  crypto/tls.(*Conn).readRecord (inline)
0     0%   100%       10ms 14.29%  crypto/tls.(*Conn).readRecordOrCCS
0     0%   100%       10ms 14.29%  crypto/tls.(*atLeastReader).Read
0     0%   100%       10ms 14.29%  encoding/json.Unmarshal
0     0%   100%       10ms 14.29%  encoding/json.checkValid
0     0%   100%       10ms 14.29%  github.com/kyma-project/telemetry-manager/controllers/telemetry.(*LogPipelineController).Reconcile
0     0%   100%       10ms 14.29%  github.com/kyma-project/telemetry-manager/internal/istiostatus.(*CheckerDiscoveryAPI).IsIstioActiveDiscoveryAPI
0     0%   100%       10ms 14.29%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).Reconcile
0     0%   100%       10ms 14.29%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).doReconcile
0     0%   100%       10ms 14.29%  github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline.(*Reconciler).reconcileFluentBit

<img src="new_pprof_with_labelselector_gardener.svg" alt="new_pprof_with_discoveryapi_gardener" width="600" height="1700"/>