# GoHome Configuration Examples

This directory contains example `main.pkl` configuration files to help you get started with gohome.

## Which file should I use?

### `minimal-main.pkl` — New to gohome?

Start here. This file shows the bare minimum configuration needed to run gohome:

- Listeners (how gohome accepts connections)
- One example entity (a light)
- Empty sections for entities, automations, dashboards, etc.

**Best for:** Getting gohome running quickly, understanding the configuration structure, beginners.

**Copy it:**
\`\`\`bash
cp minimal-main.pkl main.pkl
# Then customize main.pkl for your setup
\`\`\`

### `full-main.pkl` — Ready to see everything?

This file demonstrates all major gohome subsystems in a realistic smart home scenario:

- Multiple entities (lights, sensors, switches across rooms)
- Automations (motion-activated lights, temperature alerts)
- Scripts (morning routine, evening shutdown)
- Dashboards (device overview)
- Authentication (users and access control)
- MCP configuration
- Detailed comments explaining each section

**Best for:** Understanding how subsystems work together, reference implementation, advanced users.

**Copy it:**
\`\`\`bash
cp full-main.pkl main.pkl
# Then customize main.pkl for your setup
\`\`\`

## Next steps

1. Copy one of these files to `main.pkl` (or your config directory)
2. Customize it for your setup:
   - Replace entity IDs with your actual device names
   - Add your devices and automations
   - Set up users/roles if needed
3. Run gohomed:
   \`\`\`bash
   gohomed --config-dir /path/to/config/directory
   \`\`\`

## Need more help?

- See the comments in each `.pkl` file for configuration guidance
- Check the main gohome documentation for detailed guides
- Refer to the Pkl language docs if you need syntax help
