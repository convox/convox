#!/usr/bin/env python3
"""Phase 2: Fix code block language tags and standardize frontmatter."""

import os
import re
import sys

DOCS_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

def slug_to_kebab(slug):
    """Convert a slug to kebab-case."""
    # Already kebab-case or snake_case (for rack params) - leave snake_case alone
    if slug == slug.lower() and ' ' not in slug and '&' not in slug:
        return slug
    # Special cases
    special = {
        'convox.yml': 'convox-yml',
        'Node.js': 'node-js',
        'macOS-M1': 'macos-m1',
        'CLI': 'cli',
    }
    if slug in special:
        return special[slug]
    # General: lowercase, replace spaces/& with hyphens
    result = slug.lower()
    result = result.replace(' & ', '-and-')
    result = result.replace('&', '-and-')
    result = result.replace(' ', '-')
    # Remove consecutive hyphens
    result = re.sub(r'-+', '-', result)
    return result


def detect_language(content):
    """Detect the appropriate language tag for a code block currently tagged as html."""
    lines = content.strip().split('\n')
    if not lines:
        return 'text'

    # Strip leading whitespace from all lines for analysis
    stripped_lines = [l.strip() for l in lines if l.strip()]
    if not stripped_lines:
        return 'text'

    first = stripped_lines[0]
    all_text = '\n'.join(stripped_lines)

    # Dockerfile
    if first.startswith('FROM ') or first.startswith('RUN ') or first.startswith('COPY '):
        return 'dockerfile'

    # JSON (starts with { or [)
    if first.startswith('{') or first.startswith('['):
        return 'json'

    # Bash commands - lines starting with $ or # (comment/shell prompt)
    if first.startswith('$ ') or first.startswith('# ') and not first.startswith('# K8s'):
        return 'bash'

    # Common CLI tools (bare commands without $)
    bash_prefixes = [
        'convox ', 'az ', 'kubectl ', 'curl ', 'export ', 'echo ',
        'pip ', 'sudo ', 'git ', 'docker ', 'minikube ', 'brew ',
        'terraform ', 'helm ', 'apt', 'wget ', 'chmod ', 'ssh ',
        'cat ', 'mkdir ', 'touch ', 'source ', 'RACK_URL=', 'ROLE_ID=',
        'SP_OBJECT_ID=',
    ]
    for prefix in bash_prefixes:
        if first.startswith(prefix):
            return 'bash'

    # Kubernetes/YAML manifests
    if 'apiVersion:' in all_text or 'kind:' in all_text:
        return 'yaml'

    # convox.yml style YAML
    yaml_indicators = ['services:', 'environment:', 'resources:', 'timers:', 'balancers:',
                       'scale:', 'build:', 'port:', 'health:', 'domain:', 'command:',
                       'image:', 'volumes:', 'labels:', 'annotations:',
                       'spec:', 'metadata:', 'selector:']
    for indicator in yaml_indicators:
        if indicator in all_text:
            return 'yaml'

    # YAML-like: lines with key: value pattern
    yaml_line_count = sum(1 for l in stripped_lines if re.match(r'^[\w_-]+:\s', l))
    if yaml_line_count >= 2:
        return 'yaml'

    # Table-like output (multiple columns separated by spaces)
    # CLI output typically has headers like NAME STATUS etc.
    if re.match(r'^[A-Z]{2,}', first) and len(stripped_lines) > 1:
        return 'text'

    # Formula/pseudocode
    if 'desiredReplicas' in first or '=' in first and not first.startswith('export'):
        return 'text'

    # URL-like content
    if first.startswith('http') or first.startswith('https'):
        return 'text'

    # Default: if it looks like a bare command
    if len(stripped_lines) == 1 and not ':' in first:
        return 'bash'

    # Output text (IDs, status messages, etc.)
    return 'text'


def fix_code_blocks(content):
    """Replace ```html with the correct language tag."""
    def replace_block(match):
        tag = match.group(1) or ''
        trailing = match.group(2) or ''  # capture trailing whitespace after tag
        block_content = match.group(3)

        if tag.strip() == 'html':
            new_tag = detect_language(block_content)
            return f'```{new_tag}\n{block_content}```'
        return match.group(0)

    # Match ```html (with optional trailing space) followed by content until ```
    pattern = r'```(html)([ \t]*)?\n(.*?)```'
    return re.sub(pattern, replace_block, content, flags=re.DOTALL)


def fix_frontmatter(content, filepath):
    """Standardize frontmatter: remove draft:false, fix slugs, fix trailing whitespace."""
    # Match frontmatter
    fm_match = re.match(r'^---\s*\n(.*?)\n---', content, re.DOTALL)
    if not fm_match:
        return content

    fm = fm_match.group(1)
    rest = content[fm_match.end():]

    lines = fm.split('\n')
    new_lines = []

    for line in lines:
        stripped = line.strip()

        # Skip blank lines in frontmatter
        if not stripped:
            continue

        # Remove draft: false
        if stripped == 'draft: false':
            continue

        # Fix slug to kebab-case
        slug_match = re.match(r'^slug:\s*(.+)', stripped)
        if slug_match:
            old_slug = slug_match.group(1).strip()
            new_slug = slug_to_kebab(old_slug)
            new_lines.append(f'slug: {new_slug}')
            continue

        # Fix trailing whitespace on url
        url_match = re.match(r'^url:\s*(.+)', stripped)
        if url_match:
            url = url_match.group(1).strip()
            new_lines.append(f'url: {url}')
            continue

        # Keep other lines, strip trailing whitespace
        new_lines.append(stripped)

    new_fm = '\n'.join(new_lines)
    return f'---\n{new_fm}\n---{rest}'


def process_file(filepath):
    """Process a single markdown file."""
    with open(filepath, 'r', encoding='utf-8') as f:
        original = f.read()

    content = original

    # Fix frontmatter
    content = fix_frontmatter(content, filepath)

    # Fix code blocks
    content = fix_code_blocks(content)

    if content != original:
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(content)
        return True
    return False


def main():
    changed = 0
    total = 0

    for root, dirs, files in os.walk(DOCS_ROOT):
        # Skip _refactor directory
        if '_refactor' in root:
            continue
        for fname in sorted(files):
            if not fname.endswith('.md'):
                continue
            filepath = os.path.join(root, fname)
            total += 1
            if process_file(filepath):
                relpath = os.path.relpath(filepath, DOCS_ROOT)
                print(f'  Fixed: {relpath}')
                changed += 1

    print(f'\nProcessed {total} files, modified {changed}')


if __name__ == '__main__':
    main()
