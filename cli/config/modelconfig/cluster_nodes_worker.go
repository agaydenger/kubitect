package modelconfig

import v "cli/validation"

type WorkerDefault struct {
	CPU          *VCpu    `yaml:"cpu"`
	RAM          *GB      `yaml:"ram"`
	MainDiskSize *GB      `yaml:"mainDiskSize"`
	Labels       *Labels  `yaml:"labels"`
	Taints       *[]Taint `yaml:"taints"`
}

func (d WorkerDefault) Validate() error {
	return v.Struct(&d,
		v.Field(&d.CPU, v.OmitEmpty()),
		v.Field(&d.RAM, v.OmitEmpty()),
		v.Field(&d.MainDiskSize, v.OmitEmpty()),
		v.Field(&d.Labels, v.OmitEmpty()), // TODO: Is omit empty required?
		v.Field(&d.Taints),
	)
}

type LabelKey string // TODO: Check if correct type

type Worker struct {
	Default   *WorkerDefault    `yaml:"default"`
	Instances *[]WorkerInstance `yaml:"instances"`
}

func (w Worker) Validate() error {
	return v.Struct(&w,
		v.Field(&w.Instances),
		v.Field(&w.Default),
	)
}

type WorkerInstance struct {
	Id           *string     `yaml:"id" opt:",id"`
	Host         *string     `yaml:"host"`
	IP           *IP         `yaml:"ip"`
	MAC          *MAC        `yaml:"mac"`
	CPU          *VCpu       `yaml:"cpu"`
	RAM          *GB         `yaml:"ram"`
	MainDiskSize *GB         `yaml:"mainDiskSize"`
	DataDisks    *[]DataDisk `yaml:"dataDisks"`
	Labels       *Labels     `yaml:"labels"`
	Taints       *[]Taint    `yaml:"taints"`
}

func (i WorkerInstance) Validate() error {
	return v.Struct(&i,
		v.Field(&i.Id, v.Required()),
		// v.Field(&i.Host), // TODO: Is valid Host?
		v.Field(&i.IP, v.OmitEmpty()), // TODO: Is within CIDR?
		v.Field(&i.MAC, v.OmitEmpty()),
		v.Field(&i.CPU, v.OmitEmpty()),
		v.Field(&i.RAM, v.OmitEmpty()),
		v.Field(&i.MainDiskSize, v.OmitEmpty()),
		v.Field(&i.DataDisks),
		v.Field(&i.Labels), // TODO: Is Omit empty required?
		v.Field(&i.Taints),
	)
}
