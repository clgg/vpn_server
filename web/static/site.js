const revealObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        entry.target.classList.add("is-visible");
      }
    }
  },
  { threshold: 0.18 }
);

document.querySelectorAll(".reveal").forEach((el) => revealObserver.observe(el));

const hero = document.querySelector(".hero-bg");
window.addEventListener(
  "scroll",
  () => {
    const offset = Math.min(window.scrollY * 0.08, 42);
    hero.style.transform = `scale(1.06) translateY(${offset}px)`;
  },
  { passive: true }
);
