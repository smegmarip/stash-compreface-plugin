# Stash Compreface Plugin - Documentation Index

**Version:** 2.0.0
**Last Updated:** 2025-11-08

---

## Documentation Overview

This directory contains comprehensive documentation for the Stash Compreface Plugin, a Go RPC plugin that provides face recognition and performer synchronization for Stash using Compreface.

---

## User Documentation

### [TESTING.md](TESTING.md)
**Purpose:** Comprehensive testing guide and procedures

**Contents:**
- Test environment setup
- Unit testing procedures
- Integration testing scenarios
- End-to-end testing workflows
- Performance testing guidelines
- Error scenario testing
- Test checklists and benchmarks

**Audience:** Developers, QA testers, plugin maintainers

---

## Technical Documentation

### [ARCHITECTURE.md](ARCHITECTURE.md)
**Purpose:** Technical architecture and design documentation

**Contents:**
- System architecture overview
- Core component descriptions
- Data flow diagrams
- Subject naming convention
- Performance features (batching, cooldown, progress)
- Error handling strategy
- Deployment architecture

**Audience:** Developers, system architects, advanced users

### [QUALITY_SERVICE.md](QUALITY_SERVICE.md)
**Purpose:** Optional Quality Service documentation

**Contents:**
- Service overview and architecture
- API endpoint specifications
- Quality assessment algorithms
- Testing results and benchmarks
- Integration with main plugin
- Docker deployment
- Troubleshooting guide

**Audience:** Developers, users needing enhanced face quality assessment

**Status:** Implemented (Phase 1.5) - Optional dependency

---

## Research Documentation

### [PHASE_5_VIDEO_ANALYSIS.md](PHASE_5_VIDEO_ANALYSIS.md)
**Purpose:** Video face recognition research and architecture decisions

**Contents:**
- Analysis of 3 implementation approaches
- Existing Python implementation review
- Advanced vision techniques (InsightFace, CLIP, YOLO)
- Comparison matrix and decision rationale
- Vision Service architecture proposal
- Performance considerations

**Audience:** Developers, researchers, decision-makers

**Status:** Research complete, Vision Service client implemented, service build deferred

**Note:** This document will be consolidated into ARCHITECTURE.md once the Vision Service is fully implemented.

---

## Quick Reference

### For New Users
1. Start with main [README.md](../README.md) in repository root
2. Follow installation instructions
3. Refer to [TESTING.md](TESTING.md) for validation

### For Developers
1. Read [ARCHITECTURE.md](ARCHITECTURE.md) for system overview
2. Review [PHASE_5_VIDEO_ANALYSIS.md](PHASE_5_VIDEO_ANALYSIS.md) for vision service design
3. Consult [QUALITY_SERVICE.md](QUALITY_SERVICE.md) if implementing quality features
4. Use [TESTING.md](TESTING.md) for test procedures

### For System Administrators
1. Review [ARCHITECTURE.md](ARCHITECTURE.md) deployment section
2. Configure according to [README.md](../README.md)
3. Use [QUALITY_SERVICE.md](QUALITY_SERVICE.md) for optional quality service setup

---

## Document Status

| Document | Status | Last Updated | Scope |
|----------|--------|--------------|-------|
| ARCHITECTURE.md | ✅ Complete | 2025-11-08 | Full system architecture |
| QUALITY_SERVICE.md | ✅ Complete | 2025-11-08 | Optional quality service |
| TESTING.md | ✅ Complete | 2025-11-08 | Testing procedures |
| PHASE_5_VIDEO_ANALYSIS.md | 🔄 Interim | 2025-11-08 | Vision service research |

**Legend:**
- ✅ Complete - Documentation is final and comprehensive
- 🔄 Interim - Will be consolidated after Vision Service implementation
- 🚧 Draft - Work in progress
- ⏸️ Deferred - Pending future work

---

## Additional Resources

### Repository Root
- **[README.md](../README.md)** - User documentation, installation, usage
- **[CLAUDE.md](../CLAUDE.md)** - Master development plan and refactor documentation
- **[build.sh](../build.sh)** - Multi-platform build script

### Main Plugin
- **[compreface-rpc.yml](../compreface-rpc.yml)** - Plugin manifest
- **[gorpc/](../gorpc/)** - Go source code

### Quality Service
- **[face-quality-service/](../face-quality-service/)** - Optional Python quality service
- **[face-quality-service/README.md](../face-quality-service/README.md)** - Quality service quick start

### Vision Service (Separate Project)
- **[stash-auto-vision/CLAUDE.md](../../stash-auto-vision/CLAUDE.md)** - Service specification (1,172 lines)
- **[stash-auto-vision/RESEARCH.md](../../stash-auto-vision/RESEARCH.md)** - Vision technology research (663 lines)

---

## Contributing

When adding new documentation:

1. **User-Facing Docs** → Place in repository root (README.md, etc.)
2. **Technical Docs** → Place in `docs/` directory
3. **Update This Index** → Add new document to relevant section above
4. **Follow Format** → Use existing documents as templates
5. **Version & Date** → Include version number and last updated date

### Documentation Standards

- **Markdown Format:** Use GitHub-flavored markdown
- **Headers:** Use hierarchical structure (##, ###, ####)
- **Code Blocks:** Use syntax highlighting (```go, ```python, etc.)
- **Examples:** Include practical, working examples
- **Audience:** Clearly state target audience
- **Status:** Mark completion status (✅ Complete, 🔄 In Progress, etc.)

---

## Support

- **Issues:** https://github.com/smegmarip/stash-compreface-plugin/issues
- **Discussions:** https://github.com/smegmarip/stash-compreface-plugin/discussions
- **Stash Discord:** https://discord.gg/stash

---

## License

This project is licensed under the GPL License - see the LICENSE.md file for details.
