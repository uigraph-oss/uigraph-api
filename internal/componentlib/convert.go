package componentlib

// ToFocalPointComponent converts a stored component to the focal-point API shape.
func ToFocalPointComponent(c Component, iconURL string) FocalPointComponent {
	fields := make([]FocalPointField, len(c.Fields))
	for i, f := range c.Fields {
		fields[i] = FocalPointField{
			ComponentFieldID: f.ID,
			Label:            f.Label,
			Type:             f.Type,
			Required:         f.Required,
			Readonly:         f.Readonly,
			Options:          f.Options,
			Order:            f.Order,
		}
	}
	return FocalPointComponent{
		ComponentID:     c.ID,
		Type:            c.Type,
		Name:            c.Name,
		Description:     c.Description,
		Category:        c.CategoryName,
		Tags:            c.Tags,
		Slug:            c.Slug,
		PreviewImageJpg: iconURL,
		IsActive:        c.IsActive,
		Order:           c.Order,
		ComponentFields: fields,
	}
}

// ToFlowDiagramComponent converts a stored component to the flow-diagram API shape.
func ToFlowDiagramComponent(c Component, iconURL string) FlowDiagramComponent {
	fields := make([]FlowDiagramField, len(c.Fields))
	for i, f := range c.Fields {
		fields[i] = FlowDiagramField{
			FlowDiagramComponentFieldID: f.ID,
			Label:                       f.Label,
			Type:                        f.Type,
			Required:                    f.Required,
			Readonly:                    f.Readonly,
			Options:                     f.Options,
			Order:                       f.Order,
		}
	}
	return FlowDiagramComponent{
		ComponentID:                c.ID,
		Type:                       c.Type,
		Name:                       c.Name,
		Description:                c.Description,
		Category:                   c.CategoryName,
		Tags:                       c.Tags,
		Slug:                       c.Slug,
		PreviewImageJpg:            iconURL,
		IsActive:                   c.IsActive,
		Order:                      c.Order,
		FlowDiagramComponentFields: fields,
	}
}
