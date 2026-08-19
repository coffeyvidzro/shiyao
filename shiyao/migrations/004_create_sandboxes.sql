CREATE TABLE IF NOT EXISTS sandboxes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Firecracker specific metadata
    vm_id VARCHAR(255) UNIQUE NOT NULL,
    template VARCHAR(50) NOT NULL DEFAULT 'python-3.11',
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, running, stopped, failed

    -- Resource limits
    vcpu INT NOT NULL DEFAULT 1,
    memory_mb INT NOT NULL DEFAULT 512,
    timeout_seconds INT NOT NULL DEFAULT 300,

    -- Networking
    allowed_hosts TEXT[] DEFAULT '{}', -- Array of allowed egress domains

    created_at TIMESTAMPTZ DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    stopped_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sandboxes_user_id ON sandboxes(user_id);
CREATE INDEX IF NOT EXISTS idx_sandboxes_status ON sandboxes(status);
CREATE INDEX idx_sandboxes_status_created ON sandboxes (status, created_at);
