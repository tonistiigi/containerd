package supervisor

type SubscribeStatsEvent struct {
	s *Supervisor
}

type UnsubscribeStatsEvent struct {
	s *Supervisor
}

type StopStatsEvent struct {
	s *Supervisor
}

type StatsEvent struct {
	s *Supervisor
}

func (h *SubscribeStatsEvent) Handle(e *Event) error {
	i, ok := h.s.containers[e.ID]
	if !ok {
		return ErrContainerNotFound
	}
	e.StatsStream = h.s.statsCollector.collect(i.container)
	return nil
}

func (h *UnsubscribeStatsEvent) Handle(e *Event) error {
	i, ok := h.s.containers[e.ID]
	if !ok {
		return ErrContainerNotFound
	}
	h.s.statsCollector.unsubscribe(i.container, e.StatsStream)
	return nil
}

func (h *StopStatsEvent) Handle(e *Event) error {
	i, ok := h.s.containers[e.ID]
	if !ok {
		return ErrContainerNotFound
	}
	h.s.statsCollector.stopCollection(i.container)
	return nil
}

func (h *StatsEvent) Handle(e *Event) error {
	i, ok := h.s.containers[e.ID]
	if !ok {
		return ErrContainerNotFound
	}
	st, err := i.container.Stats()
	if err != nil {
		return err
	}
	e.Stats = convertToPb(st)
	return nil
}
