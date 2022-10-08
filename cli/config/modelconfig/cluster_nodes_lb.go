package modelconfig

import v "cli/validation"

type LBDefault struct {
	CPU          *VCpu `yaml:"cpu"`
	RAM          *GB   `yaml:"ram"`
	MainDiskSize *GB   `yaml:"mainDiskSize"`
}

func (def LBDefault) Validate() error {
	return v.Struct(&def,
		v.Field(&def.CPU, v.OmitEmpty()),
		v.Field(&def.RAM, v.OmitEmpty()),
		v.Field(&def.MainDiskSize, v.OmitEmpty()),
	)
}

type LB struct {
	VIP             *IP                `yaml:"vip"`
	VirtualRouterId *LBVirtualRouterID `yaml:"virtualRouterId"`
	Default         *LBDefault         `yaml:"default"`
	Instances       *[]LBInstance      `yaml:"instances"`
	ForwardPorts    *[]LBPortForward   `yaml:"forwardPorts"`
}

func (lb LB) Validate() error {
	return v.Struct(&lb,
		v.Field(&lb.VIP, v.Required().When(len(*lb.Instances) > 0).Error("Virtual IP (VIP) is required when multiple load balancer instances are configured.")),
		v.Field(&lb.VirtualRouterId),
		v.Field(&lb.Default),
		v.Field(&lb.Instances),
		v.Field(&lb.ForwardPorts),
	)
}

type LBVirtualRouterID int

func (id LBVirtualRouterID) Validate() error {
	return v.Var(int(id), v.Min(0), v.Max(255))
}

type LBPortForward struct {
	Name       *string              `yaml:"name"`
	Port       *Port                `yaml:"port"`
	TargetPort *Port                `yaml:"targetPort"`
	Target     *LBPortForwardTarget `yaml:"target"`
}

func (pf LBPortForward) Validate() error {
	return v.Struct(&pf,
		v.Field(&pf.Name, v.Required(), v.AlphaNumericHypUS()),
		v.Field(&pf.Port, v.Required()),
		v.Field(&pf.TargetPort, v.OmitEmpty()),
		v.Field(&pf.Target),
	)
}

type LBPortForwardTarget string

const (
	WORKERS LBPortForwardTarget = "workers"
	MASTERS LBPortForwardTarget = "masters"
	ALL     LBPortForwardTarget = "all"
)

func (pft LBPortForwardTarget) Validate() error {
	return v.Var(pft, v.OmitEmpty(), v.OneOf(WORKERS, MASTERS, ALL))
}

type LBInstance struct {
	Id           *string     `yaml:"id" opt:",id"`
	Host         *string     `yaml:"host"`
	IP           *IP         `yaml:"ip"`
	MAC          *MAC        `yaml:"mac"`
	CPU          *VCpu       `yaml:"cpu"`
	RAM          *GB         `yaml:"ram"`
	MainDiskSize *GB         `yaml:"mainDiskSize"`
	Priority     *LBPriority `yaml:"priority"`
}

func (i LBInstance) Validate() error {
	return v.Struct(&i,
		v.Field(&i.Id, v.Required()),
		// v.Field(&i.Host, v.OmitEmpty()), // TODO: Is valid Hostname?
		v.Field(&i.IP, v.OmitEmpty()), // TODO: Is withing CIDR?
		v.Field(&i.MAC, v.OmitEmpty()),
		v.Field(&i.CPU, v.OmitEmpty()),
		v.Field(&i.RAM, v.OmitEmpty()),
		v.Field(&i.MainDiskSize, v.OmitEmpty()),
		v.Field(&i.Priority, v.OmitEmpty()),
	)
}

type LBPriority int

func (p LBPriority) Validate() error {
	return v.Var(p, v.Min(0), v.Max(255))
}
