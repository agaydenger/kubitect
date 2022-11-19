package actions

import (
	"cli/cmp"
	"cli/env"
	"cli/utils"
)

type EventType string

const (
	OK         EventType = "ok"
	SCALE_UP   EventType = "scale_up"
	SCALE_DOWN EventType = "scale_down"

	// WARN change requires user permission to continue.
	WARN EventType = "warn"

	// BLOCK change prevents further actions on the cluster.
	BLOCK EventType = "block"
)

type Event struct {
	eType   EventType
	msg     string
	path    string
	paths   []string
	changes []cmp.Change
	action  cmp.ActionType
}

func (e Event) Paths() []string {
	if len(e.path) > 0 {
		return []string{e.path}
	}

	return e.paths
}

func (e Event) Action() cmp.ActionType {
	return e.action
}

type Events []Event

// Add adds the event with the corresponding change to the list.
// If an event with a matching action and path already exists in
// the list then the change is appended to the existing event.
func (es *Events) Add(event Event, c cmp.Change) {
	for i, e := range *es {
		if e.action == event.action && e.path == event.path {
			(*es)[i].changes = append((*es)[i].changes, c)
			return
		}
	}

	event.changes = []cmp.Change{c}
	*es = append(*es, event)
}

// OfType returns events matching the given type.
func (es Events) OfType(t EventType) Events {
	var events Events

	for _, e := range es {
		if e.eType == t {
			events = append(events, e)
		}
	}

	return events
}

// Errors converts events to the utils.Errors.
func (es Events) Errors() utils.Errors {
	var err utils.Errors

	for _, e := range es {
		var paths []string

		for _, c := range e.changes {
			paths = append(paths, c.Path)
		}

		switch e.eType {
		case WARN:
			err = append(err, NewConfigChangeWarning(e.msg, paths...))
		case BLOCK:
			err = append(err, NewConfigChangeError(e.msg, paths...))
		}
	}

	return err
}

// triggerEvents returns triggered events of the corresponding action.
func triggerEvents(diff *cmp.DiffNode, action env.ApplyAction) Events {
	var trig Events

	events := events(action)

	cmp.TriggerEventsF(diff, events, trig.Add)
	cc := cmp.ConflictingChanges(diff, events)

	if len(cc) > 0 {
		trig = append(trig, Event{
			eType:   BLOCK,
			msg:     "Disallowed changes.",
			changes: cc,
		})
	}

	return trig
}

// events returns events of the corresponding action.
func events(a env.ApplyAction) []Event {
	switch a {
	case env.CREATE:
		return ModifyEvents
	case env.SCALE:
		return ScaleEvents
	case env.UPGRADE:
		return UpgradeEvents
	default:
		return nil
	}
}

// Events
var (
	UpgradeEvents = []Event{
		{
			eType: OK,
			path:  "Kubernetes.Version",
		},
		{
			eType: OK,
			path:  "Kubernetes.Kubespray.Version",
		},
	}

	ScaleEvents = []Event{
		{
			eType:  SCALE_DOWN,
			action: cmp.DELETE,
			path:   "Cluster.Nodes.Worker.Instances.*",
		},
		{
			eType:  SCALE_UP,
			action: cmp.CREATE,
			path:   "Cluster.Nodes.Worker.Instances.*",
		},
		{
			eType:  SCALE_DOWN,
			action: cmp.DELETE,
			path:   "Cluster.Nodes.LoadBalancer.Instances.*",
		},
		{
			eType:  SCALE_UP,
			action: cmp.CREATE,
			path:   "Cluster.Nodes.LoadBalancer.Instances.*",
		},
	}

	ModifyEvents = []Event{
		// Warn data destructive host changes
		{
			eType:  WARN,
			action: cmp.MODIFY,
			path:   "Hosts.*.MainResourcePoolPath",
			msg:    "Changing main resource pool location will trigger recreation of all resources bound to that resource pool, such as virtual machines and data disks.",
		},
		{
			eType:  WARN,
			action: cmp.DELETE,
			path:   "Hosts.*.DataResourcePools.*",
			msg:    "Removing data resource pool will destroy all the data on that location.",
		},
		{
			eType:  WARN,
			action: cmp.MODIFY,
			path:   "Hosts.*.DataResourcePools.*.Path",
			msg:    "Changing data resource pool location will trigger recreation of all resources bound to that resource pool, such as virtual machines and data disks",
		},
		// Allow other host changes
		{
			eType: OK,
			path:  "Hosts",
		},
		// Prevent cluster network changes
		{
			eType: BLOCK,
			path:  "Cluster.Network",
			msg:   "Once the cluster is created, further changes to the network properties are not allowed. Such action may render the cluster unusable.",
		},
		// Prevent nodeTemplate changes
		{
			eType: BLOCK,
			path:  "Cluster.NodeTemplate",
			msg:   "Once the cluster is created, further changes to the nodeTemplate properties are not allowed. Such action may render the cluster unusable.",
		},
		// Prevent removing nodes
		{
			eType:  BLOCK,
			action: cmp.DELETE,
			paths: []string{
				"Cluster.Nodes.LoadBalancer.Instances.*",
				"Cluster.Nodes.Worker.Instances.*",
				"Cluster.Nodes.Master.Instances.*",
			},
			msg: "To remove existing nodes run apply command with '--action scale' flag.",
		},
		// Prevent adding nodes
		{
			eType:  BLOCK,
			action: cmp.CREATE,
			paths: []string{
				"Cluster.Nodes.LoadBalancer.Instances.*",
				"Cluster.Nodes.Worker.Instances.*",
				"Cluster.Nodes.Master.Instances.*",
			},
			msg: "To add new nodes run apply command with '--action scale' flag.",
		},
		// Prevent default CPU, RAM and main disk size changes
		{
			eType: BLOCK,
			paths: []string{
				"Cluster.Nodes.Worker.Default.CPU",
				"Cluster.Nodes.Worker.Default.RAM",
				"Cluster.Nodes.Worker.Default.MainDiskSize",
				"Cluster.Nodes.Master.Default.CPU",
				"Cluster.Nodes.Master.Default.RAM",
				"Cluster.Nodes.Master.Default.MainDiskSize",
				"Cluster.Nodes.LoadBalancer.Default.CPU",
				"Cluster.Nodes.LoadBalancer.Default.RAM",
				"Cluster.Nodes.LoadBalancer.Default.MainDiskSize",
			},
			msg: "Changing any default physical properties of nodes (cpu, ram, mainDiskSize) is not allowed. Such action may render the cluster unusable.",
		},
		// Prevent CPU, RAM and main disk size changes
		{
			eType:  BLOCK,
			action: cmp.MODIFY,
			paths: []string{
				"Cluster.Nodes.Worker.Instances.*.CPU",
				"Cluster.Nodes.Worker.Instances.*.RAM",
				"Cluster.Nodes.Worker.Instances.*.MainDiskSize",
				"Cluster.Nodes.Master.Instances.*.CPU",
				"Cluster.Nodes.Master.Instances.*.RAM",
				"Cluster.Nodes.Master.Instances.*.MainDiskSize",
				"Cluster.Nodes.LoadBalancer.Instances.*.CPU",
				"Cluster.Nodes.LoadBalancer.Instances.*.RAM",
				"Cluster.Nodes.LoadBalancer.Instances.*.MainDiskSize",
			},
			msg: "Changing any physical properties of nodes (cpu, ram, mainDiskSize) is not allowed. Such action will recreate the node.",
		},
		// Prevent IP and MAC changes
		{
			eType:  BLOCK,
			action: cmp.MODIFY,
			paths: []string{
				"Cluster.Nodes.Worker.Instances.*.IP",
				"Cluster.Nodes.Worker.Instances.*.MAC",
				"Cluster.Nodes.Master.Instances.*.IP",
				"Cluster.Nodes.Master.Instances.*.MAC",
				"Cluster.Nodes.LoadBalancer.Instances.*.IP",
				"Cluster.Nodes.LoadBalancer.Instances.*.MAC",
			},
			msg: "Changing IP or MAC address of the node is not allowed. Such action may render the cluster unusable.",
		},
		// Data disk changes
		{
			eType:  WARN,
			action: cmp.MODIFY,
			paths: []string{
				"Cluster.Nodes.Worker.Instances.*.DataDisks.*",
				"Cluster.Nodes.Master.Instances.*.DataDisks.*",
			},
			msg: "Changing data disk properties, will recreate the disk (removing all of its content in the process).",
		},
		{
			eType:  WARN,
			action: cmp.DELETE,
			paths: []string{
				"Cluster.Nodes.Master.Instances.*.DataDisks.*",
				"Cluster.Nodes.Worker.Instances.*.DataDisks.*",
			},
			msg: "One or more data disks will be removed.",
		},
		{
			eType:  OK,
			action: cmp.CREATE,
			paths: []string{
				"Cluster.Nodes.Master.Instances.*.DataDisks.*",
				"Cluster.Nodes.Worker.Instances.*.DataDisks.*",
			},
		},
		// Prevent VIP changes
		{
			eType: BLOCK,
			path:  "Cluster.Nodes.LoadBalancer.VIP",
			msg:   "Once the cluster is created, changing virtual IP (VIP) is not allowed. Such action may render the cluster unusable.",
		},
		// Allow all other node properties to be changed
		{
			eType: OK,
			paths: []string{
				"Cluster.Nodes.Master.Instances.*",
				"Cluster.Nodes.Worker.Instances.*",
				"Cluster.Nodes.LoadBalancer.Instances.*",
			},
		},
		// Prevent k8s properties changes
		{
			eType: BLOCK,
			paths: []string{
				"Kubernetes.Version",
				"Kubernetes.Kubespray.Version",
			},
			msg: "Changing Kubernetes or Kubespray version is allowed only when upgrading the cluster.\nTo upgrade the cluster run apply command with '--action upgrade' flag.",
		},
		// Allow addons changes
		{
			eType: OK,
			path:  "Addons",
		},
		// Allow kubitect (project metadata) changes
		{
			eType: OK,
			path:  "Kubitect",
		},
	}
)
