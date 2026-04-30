package auth

// PeerCred holds Unix peer credentials obtained via SO_PEERCRED.
// Using a package-defined type avoids a direct dependency on syscall.Ucred,
// which is Linux-specific.
type PeerCred struct {
	Pid int32
	Uid uint32
	Gid uint32
}
