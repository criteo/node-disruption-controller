# API Reference

Packages:

- [nodedisruption.criteo.com/v1alpha1](#nodedisruptioncriteocomv1alpha1)

# nodedisruption.criteo.com/v1alpha1

Resource Types:

- [ApplicationDisruptionBudget](#applicationdisruptionbudget)

- [NodeDisruptionBudget](#nodedisruptionbudget)

- [NodeDisruption](#nodedisruption)




## ApplicationDisruptionBudget
<sup><sup>[↩ Parent](#nodedisruptioncriteocomv1alpha1 )</sup></sup>






ApplicationDisruptionBudget is the Schema for the applicationdisruptionbudgets API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>nodedisruption.criteo.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>ApplicationDisruptionBudget</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspec">spec</a></b></td>
        <td>object</td>
        <td>
          ApplicationDisruptionBudgetSpec defines the desired state of ApplicationDisruptionBudget<br/>
          <br/>
            <i>Validations</i>:<li>!(has(self.hookV2BasePath) && has(self.hookV2BasePath.url) && size(self.hookV2BasePath.url) > 0 && has(self.hookV3BasePath) && has(self.hookV3BasePath.url) && size(self.hookV3BasePath.url) > 0): hookV2BasePath and hookV3BasePath are mutually exclusive</li>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetstatus">status</a></b></td>
        <td>object</td>
        <td>
          DisruptionBudgetStatus defines the observed state of ApplicationDisruptionBudget<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec
<sup><sup>[↩ Parent](#applicationdisruptionbudget)</sup></sup>



ApplicationDisruptionBudgetSpec defines the desired state of ApplicationDisruptionBudget

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>maxDisruptions</b></td>
        <td>integer</td>
        <td>
          A NodeDisruption is allowed if at most "maxDisruptions" nodes selected by selectors are unavailable after the disruption.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspechealthhook">healthHook</a></b></td>
        <td>object</td>
        <td>
          Define a optional hook to call when validating a NodeDisruption.
It perform a POST http request containing the NodeDisruption that is being validated.
Maintenance will proceed only if the endpoint responds 2XX.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspechookv2basepath">hookV2BasePath</a></b></td>
        <td>object</td>
        <td>
          HookV2BasePath holds the base path for the prepare, ready, terminate hooks that will be
called at different stages of the NodeDisruption lifecycle.
A POST http request containing a Disruption that is being reconciled is sent ot each of the hooks.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspechookv3basepath">hookV3BasePath</a></b></td>
        <td>object</td>
        <td>
          HookV3BasePath holds the base path for the accept, prepare, ready, terminate hooks that will be
called at different stages of the NodeDisruption lifecycle. It is the v2 workflow with an
additional accept step: prepare is only called once every budget impacted by the NodeDisruption
has accepted it, so that applications sharing nodes prepare together.
A POST http request containing a Disruption that is being reconciled is sent to each of the hooks.
Mutually exclusive with HookV2BasePath.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>noticePeriod</b></td>
        <td>string</td>
        <td>
          NoticePeriod is the notice this application wants before a disruption. Hooks for this
budget are not called, and the disruption is not granted, before the disruption's
spec.startDate − noticePeriod. A disruption without startDate is not deferred, nor is one
with retry enabled whose retry deadline is at or before that time. Notice periods are
whole hours, minutes and seconds, such as 30m or 1h30m.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspecnoticeperiodoverridesindex">noticePeriodOverrides</a></b></td>
        <td>[]object</td>
        <td>
          NoticePeriodOverrides overrides NoticePeriod for specific disruption types.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspecpodselector">podSelector</a></b></td>
        <td>object</td>
        <td>
          PodSelector query over pods whose nodes are managed by the disruption budget.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetspecpvcselector">pvcSelector</a></b></td>
        <td>object</td>
        <td>
          PVCSelector query over PVCs whose nodes are managed by the disruption budget.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>supportedNodeDisruptionTypes</b></td>
        <td>[]string</td>
        <td>
          SupportedNodeDisruptionTypes is the list of node disruption types that this budget supports.
When set, this budget will only be considered during reconciliation of NodeDisruptions whose type
is in this list. When empty, the controller's default node disruption types are used.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.healthHook
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspec)</sup></sup>



Define a optional hook to call when validating a NodeDisruption.
It perform a POST http request containing the NodeDisruption that is being validated.
Maintenance will proceed only if the endpoint responds 2XX.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>caBundle</b></td>
        <td>string</td>
        <td>
          a PEM encoded CA bundle which will be used to validate the webhook's server certificate. If unspecified, system trust roots on the apiserver are used.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>url</b></td>
        <td>string</td>
        <td>
          URL that will be called by the hook, in standard URL form (`scheme://host:port/path`).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.hookV2BasePath
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspec)</sup></sup>



HookV2BasePath holds the base path for the prepare, ready, terminate hooks that will be
called at different stages of the NodeDisruption lifecycle.
A POST http request containing a Disruption that is being reconciled is sent ot each of the hooks.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>caBundle</b></td>
        <td>string</td>
        <td>
          a PEM encoded CA bundle which will be used to validate the webhook's server certificate. If unspecified, system trust roots on the apiserver are used.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>url</b></td>
        <td>string</td>
        <td>
          URL that will be called by the hook, in standard URL form (`scheme://host:port/path`).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.hookV3BasePath
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspec)</sup></sup>



HookV3BasePath holds the base path for the accept, prepare, ready, terminate hooks that will be
called at different stages of the NodeDisruption lifecycle. It is the v2 workflow with an
additional accept step: prepare is only called once every budget impacted by the NodeDisruption
has accepted it, so that applications sharing nodes prepare together.
A POST http request containing a Disruption that is being reconciled is sent to each of the hooks.
Mutually exclusive with HookV2BasePath.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>caBundle</b></td>
        <td>string</td>
        <td>
          a PEM encoded CA bundle which will be used to validate the webhook's server certificate. If unspecified, system trust roots on the apiserver are used.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>url</b></td>
        <td>string</td>
        <td>
          URL that will be called by the hook, in standard URL form (`scheme://host:port/path`).<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.noticePeriodOverrides[index]
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspec)</sup></sup>



TypedNoticePeriod is a notice period for a single node disruption type.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>noticePeriod</b></td>
        <td>string</td>
        <td>
          NoticePeriod for disruptions of this type.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type is the node disruption type this notice period applies to.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.podSelector
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspec)</sup></sup>



PodSelector query over pods whose nodes are managed by the disruption budget.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#applicationdisruptionbudgetspecpodselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
map is equivalent to an element of matchExpressions, whose key field is "key", the
operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.podSelector.matchExpressions[index]
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspecpodselector)</sup></sup>



A label selector requirement is a selector that contains values, a key, and an operator that
relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.
Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn,
the values array must be non-empty. If the operator is Exists or DoesNotExist,
the values array must be empty. This array is replaced during a strategic
merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.pvcSelector
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspec)</sup></sup>



PVCSelector query over PVCs whose nodes are managed by the disruption budget.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#applicationdisruptionbudgetspecpvcselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
map is equivalent to an element of matchExpressions, whose key field is "key", the
operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.spec.pvcSelector.matchExpressions[index]
<sup><sup>[↩ Parent](#applicationdisruptionbudgetspecpvcselector)</sup></sup>



A label selector requirement is a selector that contains values, a key, and an operator that
relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.
Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn,
the values array must be non-empty. If the operator is Exists or DoesNotExist,
the values array must be empty. This array is replaced during a strategic
merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.status
<sup><sup>[↩ Parent](#applicationdisruptionbudget)</sup></sup>



DisruptionBudgetStatus defines the observed state of ApplicationDisruptionBudget

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>currentDisruptions</b></td>
        <td>integer</td>
        <td>
          Number of disruption currently seen on the cluster<br/>
          <br/>
            <i>Default</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#applicationdisruptionbudgetstatusdisruptionsindex">disruptions</a></b></td>
        <td>[]object</td>
        <td>
          Disruptions contains a list of disruptions that are related to the budget<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>disruptionsAllowed</b></td>
        <td>integer</td>
        <td>
          Number of disruption allowed on the nodes of this<br/>
          <br/>
            <i>Default</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>watchedNodes</b></td>
        <td>[]string</td>
        <td>
          List of nodes that are being watched by the controller
Disruption on this nodes will will be made according to the budget
of this cluster.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### ApplicationDisruptionBudget.status.disruptions[index]
<sup><sup>[↩ Parent](#applicationdisruptionbudgetstatus)</sup></sup>



Basic information about disruptions

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the disruption<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State of the disruption<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>

## NodeDisruptionBudget
<sup><sup>[↩ Parent](#nodedisruptioncriteocomv1alpha1 )</sup></sup>






NodeDisruptionBudget is the Schema for the nodedisruptionbudgets API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>nodedisruption.criteo.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>NodeDisruptionBudget</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetspec">spec</a></b></td>
        <td>object</td>
        <td>
          NodeDisruptionBudgetSpec defines the desired state of NodeDisruptionBudget<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetstatus">status</a></b></td>
        <td>object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.spec
<sup><sup>[↩ Parent](#nodedisruptionbudget)</sup></sup>



NodeDisruptionBudgetSpec defines the desired state of NodeDisruptionBudget

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>maxDisruptedNodes</b></td>
        <td>integer</td>
        <td>
          A NodeDisruption is allowed if at most "maxDisruptedNodes" nodes selected by selectors are unavailable after the disruption.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>minUndisruptedNodes</b></td>
        <td>integer</td>
        <td>
          A NodeDisruption is allowed if at most "minUndisruptedNodes" nodes selected by selectors are unavailable after the disruption.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetspecnodeselector">nodeSelector</a></b></td>
        <td>object</td>
        <td>
          NodeSelector query over pods whose nodes are managed by the disruption budget.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>supportedNodeDisruptionTypes</b></td>
        <td>[]string</td>
        <td>
          SupportedNodeDisruptionTypes is the list of node disruption types that this budget supports.
When set, this budget will only be considered during reconciliation of NodeDisruptions whose type
is in this list. When empty, the controller's default node disruption types are used.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetspectaintselector">taintSelector</a></b></td>
        <td>object</td>
        <td>
          TaintSelector allow to select taints that will be considered disruptions<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.spec.nodeSelector
<sup><sup>[↩ Parent](#nodedisruptionbudgetspec)</sup></sup>



NodeSelector query over pods whose nodes are managed by the disruption budget.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#nodedisruptionbudgetspecnodeselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
map is equivalent to an element of matchExpressions, whose key field is "key", the
operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.spec.nodeSelector.matchExpressions[index]
<sup><sup>[↩ Parent](#nodedisruptionbudgetspecnodeselector)</sup></sup>



A label selector requirement is a selector that contains values, a key, and an operator that
relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.
Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn,
the values array must be non-empty. If the operator is Exists or DoesNotExist,
the values array must be empty. This array is replaced during a strategic
merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.spec.taintSelector
<sup><sup>[↩ Parent](#nodedisruptionbudgetspec)</sup></sup>



TaintSelector allow to select taints that will be considered disruptions

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#nodedisruptionbudgetspectaintselectorexcludeindex">exclude</a></b></td>
        <td>[]object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetspectaintselectorincludeindex">include</a></b></td>
        <td>[]object</td>
        <td>
          <br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.spec.taintSelector.exclude[index]
<sup><sup>[↩ Parent](#nodedisruptionbudgetspectaintselector)</sup></sup>



The pod this Toleration is attached to tolerates any taint that matches
the triple <key,value,effect> using the matching operator <operator>.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>effect</b></td>
        <td>string</td>
        <td>
          Effect indicates the taint effect to match. Empty means match all taint effects.
When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key is the taint key that the toleration applies to. Empty means match all taint keys.
If the key is empty, operator must be Exists; this combination means to match all values and all keys.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Operator represents a key's relationship to the value.
Valid operators are Exists and Equal. Defaults to Equal.
Exists is equivalent to wildcard for value, so that a pod can
tolerate all taints of a particular category.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>tolerationSeconds</b></td>
        <td>integer</td>
        <td>
          TolerationSeconds represents the period of time the toleration (which must be
of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default,
it is not set, which means tolerate the taint forever (do not evict). Zero and
negative values will be treated as 0 (evict immediately) by the system.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value is the taint value the toleration matches to.
If the operator is Exists, the value should be empty, otherwise just a regular string.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.spec.taintSelector.include[index]
<sup><sup>[↩ Parent](#nodedisruptionbudgetspectaintselector)</sup></sup>



The pod this Toleration is attached to tolerates any taint that matches
the triple <key,value,effect> using the matching operator <operator>.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>effect</b></td>
        <td>string</td>
        <td>
          Effect indicates the taint effect to match. Empty means match all taint effects.
When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key is the taint key that the toleration applies to. Empty means match all taint keys.
If the key is empty, operator must be Exists; this combination means to match all values and all keys.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          Operator represents a key's relationship to the value.
Valid operators are Exists and Equal. Defaults to Equal.
Exists is equivalent to wildcard for value, so that a pod can
tolerate all taints of a particular category.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>tolerationSeconds</b></td>
        <td>integer</td>
        <td>
          TolerationSeconds represents the period of time the toleration (which must be
of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default,
it is not set, which means tolerate the taint forever (do not evict). Zero and
negative values will be treated as 0 (evict immediately) by the system.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>value</b></td>
        <td>string</td>
        <td>
          Value is the taint value the toleration matches to.
If the operator is Exists, the value should be empty, otherwise just a regular string.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.status
<sup><sup>[↩ Parent](#nodedisruptionbudget)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>currentDisruptions</b></td>
        <td>integer</td>
        <td>
          Number of disruption currently seen on the cluster<br/>
          <br/>
            <i>Default</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetstatusdisruptionsindex">disruptions</a></b></td>
        <td>[]object</td>
        <td>
          Disruptions contains a list of disruptions that are related to the budget<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>disruptionsAllowed</b></td>
        <td>integer</td>
        <td>
          Number of disruption allowed on the nodes of this<br/>
          <br/>
            <i>Default</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionbudgetstatustaintednodesindex">taintedNodes</a></b></td>
        <td>[]object</td>
        <td>
          TaintedNodes contains nodes selected that have an allowed taint<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>watchedNodes</b></td>
        <td>[]string</td>
        <td>
          List of nodes that are being watched by the controller
Disruption on this nodes will will be made according to the budget
of this cluster.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.status.disruptions[index]
<sup><sup>[↩ Parent](#nodedisruptionbudgetstatus)</sup></sup>



Basic information about disruptions

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          Name of the disruption<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>string</td>
        <td>
          State of the disruption<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### NodeDisruptionBudget.status.taintedNodes[index]
<sup><sup>[↩ Parent](#nodedisruptionbudgetstatus)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>taints</b></td>
        <td>[]string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>

## NodeDisruption
<sup><sup>[↩ Parent](#nodedisruptioncriteocomv1alpha1 )</sup></sup>






NodeDisruption is the Schema for the nodedisruptions API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>nodedisruption.criteo.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>NodeDisruption</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionspec">spec</a></b></td>
        <td>object</td>
        <td>
          NodeDisruptionSpec defines the desired state of NodeDisruption<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionstatus">status</a></b></td>
        <td>object</td>
        <td>
          NodeDisruptionStatus defines the observed state of NodeDisruption (/!\ it is eventually consistent)<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.spec
<sup><sup>[↩ Parent](#nodedisruption)</sup></sup>



NodeDisruptionSpec defines the desired state of NodeDisruption

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>doNotGrantBefore</b></td>
        <td>string</td>
        <td>
          DoNotGrantBefore prevents the disruption from being granted before the specified time<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>duration</b></td>
        <td>string</td>
        <td>
          Duration of the disruption once granted<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionspecnodeselector">nodeSelector</a></b></td>
        <td>object</td>
        <td>
          Label query over nodes that will be impacted by the disruption<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionspecretry">retry</a></b></td>
        <td>object</td>
        <td>
          Retry configuration allows to retry a disruption after rejection until the given deadline<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>startDate</b></td>
        <td>string</td>
        <td>
          StartDate when the disruption should start<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          Type of the node disruption<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.spec.nodeSelector
<sup><sup>[↩ Parent](#nodedisruptionspec)</sup></sup>



Label query over nodes that will be impacted by the disruption

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#nodedisruptionspecnodeselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
map is equivalent to an element of matchExpressions, whose key field is "key", the
operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.spec.nodeSelector.matchExpressions[index]
<sup><sup>[↩ Parent](#nodedisruptionspecnodeselector)</sup></sup>



A label selector requirement is a selector that contains values, a key, and an operator that
relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.
Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn,
the values array must be non-empty. If the operator is Exists or DoesNotExist,
the values array must be empty. This array is replaced during a strategic
merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.spec.retry
<sup><sup>[↩ Parent](#nodedisruptionspec)</sup></sup>



Retry configuration allows to retry a disruption after rejection until the given deadline

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>deadline</b></td>
        <td>string</td>
        <td>
          Deadline after which the disruption is not retried<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>enabled</b></td>
        <td>boolean</td>
        <td>
          Enable retrying<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.status
<sup><sup>[↩ Parent](#nodedisruption)</sup></sup>



NodeDisruptionStatus defines the observed state of NodeDisruption (/!\ it is eventually consistent)

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#nodedisruptionstatusdisrupteddisruptionbudgetsindex">disruptedDisruptionBudgets</a></b></td>
        <td>[]object</td>
        <td>
          List of all the budgets disrupted by the NodeDisruption<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>disruptedNodes</b></td>
        <td>[]string</td>
        <td>
          List of all the nodes that are disrupted by this NodeDisruption<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>nextRetryDate</b></td>
        <td>string</td>
        <td>
          Date of the next attempt<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>state</b></td>
        <td>enum</td>
        <td>
          Disruption status<br/>
          <br/>
            <i>Enum</i>: pending, granted, rejected<br/>
            <i>Default</i>: pending<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.status.disruptedDisruptionBudgets[index]
<sup><sup>[↩ Parent](#nodedisruptionstatus)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>ok</b></td>
        <td>boolean</td>
        <td>
          Ok is set to true when application manager preparation is complete and ready for the NodeDisruption to continue.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>preparing</b></td>
        <td>boolean</td>
        <td>
          Preparing is set to true when the application manager started preparation. Value is kept true even when the app is ready.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>accepted</b></td>
        <td>boolean</td>
        <td>
          Accepted is set to true when the application manager of a budget using v3 hooks accepted the disruption.
Preparation starts only once every impacted budget accepted it.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#nodedisruptionstatusdisrupteddisruptionbudgetsindexreference">reference</a></b></td>
        <td>object</td>
        <td>
          This is the same as types.NamespacedName but serialisable to JSON<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>waiting</b></td>
        <td>boolean</td>
        <td>
          Waiting is set to true while the budget waits for its notice period: its hooks have not been called yet.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### NodeDisruption.status.disruptedDisruptionBudgets[index].reference
<sup><sup>[↩ Parent](#nodedisruptionstatusdisrupteddisruptionbudgetsindex)</sup></sup>



This is the same as types.NamespacedName but serialisable to JSON

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          <br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>
