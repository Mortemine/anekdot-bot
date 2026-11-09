type Props = {
  filled?: boolean;
};

const frame = {
  width: 16,
  height: 16,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "round",
  strokeLinejoin: "round",
  "aria-hidden": "true",
  focusable: "false",
} as const;

export function IconHeart({ filled = false }: Props) {
  return (
    <svg {...frame} fill={filled ? "currentColor" : "none"}>
      <path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z" />
    </svg>
  );
}

export function IconBookmark({ filled = false }: Props) {
  return (
    <svg {...frame} fill={filled ? "currentColor" : "none"}>
      <path d="M19 21l-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2Z" />
    </svg>
  );
}

export function IconFolderPlus() {
  return (
    <svg {...frame}>
      <path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z" />
      <path d="M12 10v6" />
      <path d="M9 13h6" />
    </svg>
  );
}

export function IconChevronsUpDown() {
  return (
    <svg {...frame}>
      <path d="m7 15 5 5 5-5" />
      <path d="m7 9 5-5 5 5" />
    </svg>
  );
}

export function IconChevronDown() {
  return (
    <svg {...frame}>
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

export function IconChevronLeft() {
  return (
    <svg {...frame}>
      <path d="m15 18-6-6 6-6" />
    </svg>
  );
}

export function IconChevronRight() {
  return (
    <svg {...frame}>
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}

export function IconUser() {
  return (
    <svg {...frame}>
      <path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2" />
      <circle cx="12" cy="7" r="4" />
    </svg>
  );
}
