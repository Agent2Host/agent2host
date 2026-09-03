package decode

// Agent decodes a Schema-valid Agent document from its original JSON bytes.
func Agent(raw []byte) (*AgentSource, error) {
	var out AgentSource
	if err := decodeBytes(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// System decodes a Schema-valid AgentSystem document from its original JSON bytes.
func System(raw []byte) (*SystemSource, error) {
	var out SystemSource
	if err := decodeBytes(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Artifact decodes a Schema-valid Artifact manifest from its original JSON bytes.
func Artifact(raw []byte) (*ArtifactManifest, error) {
	var out ArtifactManifest
	if err := decodeBytes(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
