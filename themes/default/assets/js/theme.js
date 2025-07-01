// SliCLI Default Theme JavaScript

(function() {
    'use strict';

    // Theme initialization
    class DefaultTheme {
        constructor() {
            this.currentSlide = 1;
            this.totalSlides = window.SLICLI.presentation.slideCount;
            this.slides = [];
            this.isNavigating = false;
            
            // Configuration
            this.config = window.SLICLI.config;
            this.transitionType = this.config.transitions.type;
            this.transitionDuration = this.config.transitions.duration;
            
            // Initialize theme
            this.init();
        }

        init() {
            // Load slides
            this.loadSlides();
            
            // Set up event listeners
            this.setupEventListeners();
            
            // Initialize features
            this.initializeFeatures();
            
            // Show first slide
            this.showSlide(1);
            
            // Handle hash navigation
            this.handleHashNavigation();
        }

        loadSlides() {
            const container = document.getElementById('slides');
            
            // Load all slides
            for (let i = 1; i <= this.totalSlides; i++) {
                const slideDiv = document.createElement('div');
                slideDiv.className = `slide slide-${i}`;
                slideDiv.dataset.slide = i;
                
                // Add transition class based on theme config
                slideDiv.classList.add(`slide-transition-${this.transitionType}`);
                
                // Load slide content via AJAX or embed
                // For now, we'll assume slides are pre-rendered
                container.appendChild(slideDiv);
                this.slides.push(slideDiv);
            }
        }

        setupEventListeners() {
            // Keyboard navigation
            document.addEventListener('keydown', this.handleKeyboard.bind(this));
            
            // Touch/swipe navigation
            this.setupTouchNavigation();
            
            // Button navigation
            if (this.config.features['navigation-arrows']) {
                const prevBtn = document.getElementById('prev-slide');
                const nextBtn = document.getElementById('next-slide');
                
                if (prevBtn) prevBtn.addEventListener('click', () => this.previousSlide());
                if (nextBtn) nextBtn.addEventListener('click', () => this.nextSlide());
            }
            
            // Window resize
            window.addEventListener('resize', this.handleResize.bind(this));
            
            // Hash change
            window.addEventListener('hashchange', this.handleHashNavigation.bind(this));
        }

        handleKeyboard(e) {
            // Prevent navigation during transitions
            if (this.isNavigating) return;
            
            switch(e.key) {
                case 'ArrowRight':
                case ' ':
                case 'PageDown':
                    e.preventDefault();
                    this.nextSlide();
                    break;
                    
                case 'ArrowLeft':
                case 'PageUp':
                    e.preventDefault();
                    this.previousSlide();
                    break;
                    
                case 'Home':
                    e.preventDefault();
                    this.showSlide(1);
                    break;
                    
                case 'End':
                    e.preventDefault();
                    this.showSlide(this.totalSlides);
                    break;
                    
                case 'f':
                case 'F':
                    if (!e.ctrlKey && !e.metaKey) {
                        e.preventDefault();
                        this.toggleFullscreen();
                    }
                    break;
                    
                case 'Escape':
                    if (document.fullscreenElement) {
                        document.exitFullscreen();
                    }
                    break;
                    
                // Number keys for direct navigation
                default:
                    if (e.key >= '0' && e.key <= '9' && !e.ctrlKey && !e.metaKey) {
                        const slideNum = parseInt(e.key);
                        if (slideNum > 0 && slideNum <= this.totalSlides) {
                            this.showSlide(slideNum);
                        }
                    }
            }
        }

        setupTouchNavigation() {
            let touchStartX = 0;
            let touchEndX = 0;
            const threshold = 50; // Minimum swipe distance
            
            document.addEventListener('touchstart', (e) => {
                touchStartX = e.changedTouches[0].screenX;
            }, { passive: true });
            
            document.addEventListener('touchend', (e) => {
                touchEndX = e.changedTouches[0].screenX;
                this.handleSwipe(touchStartX, touchEndX, threshold);
            }, { passive: true });
        }

        handleSwipe(startX, endX, threshold) {
            const diff = startX - endX;
            
            if (Math.abs(diff) > threshold) {
                if (diff > 0) {
                    // Swipe left - next slide
                    this.nextSlide();
                } else {
                    // Swipe right - previous slide
                    this.previousSlide();
                }
            }
        }

        initializeFeatures() {
            // Progress bar
            if (this.config.features['progress-bar']) {
                this.updateProgressBar();
            }
            
            // Slide numbers
            if (this.config.features['slide-numbers']) {
                this.updateSlideNumbers();
            }
            
            // Preload adjacent slides for smooth transitions
            this.preloadAdjacentSlides();
        }

        showSlide(slideNumber) {
            if (slideNumber < 1 || slideNumber > this.totalSlides || this.isNavigating) {
                return;
            }
            
            // Set navigation flag
            this.isNavigating = true;
            
            // Hide current slide
            const currentSlideEl = this.slides[this.currentSlide - 1];
            if (currentSlideEl) {
                currentSlideEl.classList.remove('active');
            }
            
            // Update current slide number
            this.currentSlide = slideNumber;
            
            // Show new slide
            const newSlideEl = this.slides[slideNumber - 1];
            if (newSlideEl) {
                newSlideEl.classList.add('active');
                
                // Load slide content if not already loaded
                if (!newSlideEl.dataset.loaded) {
                    this.loadSlideContent(slideNumber, newSlideEl);
                }
            }
            
            // Update UI elements
            this.updateProgressBar();
            this.updateSlideNumbers();
            this.updateNavigationButtons();
            this.updateHash();
            
            // Preload adjacent slides
            this.preloadAdjacentSlides();
            
            // Reset navigation flag after transition
            setTimeout(() => {
                this.isNavigating = false;
            }, this.transitionDuration);
            
            // Emit custom event
            this.emitSlideChangeEvent(slideNumber);
        }

        loadSlideContent(slideNumber, slideElement) {
            // In a real implementation, this would fetch the slide content
            // For now, mark as loaded
            slideElement.dataset.loaded = 'true';
        }

        nextSlide() {
            if (this.currentSlide < this.totalSlides) {
                this.showSlide(this.currentSlide + 1);
            }
        }

        previousSlide() {
            if (this.currentSlide > 1) {
                this.showSlide(this.currentSlide - 1);
            }
        }

        updateProgressBar() {
            const progressFill = document.querySelector('.progress-fill');
            if (progressFill) {
                const progress = (this.currentSlide / this.totalSlides) * 100;
                progressFill.style.width = `${progress}%`;
            }
        }

        updateSlideNumbers() {
            const currentEl = document.getElementById('current-slide');
            if (currentEl) {
                currentEl.textContent = this.currentSlide;
            }
        }

        updateNavigationButtons() {
            const prevBtn = document.getElementById('prev-slide');
            const nextBtn = document.getElementById('next-slide');
            
            if (prevBtn) {
                prevBtn.disabled = this.currentSlide === 1;
            }
            
            if (nextBtn) {
                nextBtn.disabled = this.currentSlide === this.totalSlides;
            }
        }

        updateHash() {
            history.replaceState(null, null, `#${this.currentSlide}`);
        }

        handleHashNavigation() {
            const hash = window.location.hash.slice(1);
            const slideNumber = parseInt(hash);
            
            if (!isNaN(slideNumber) && slideNumber >= 1 && slideNumber <= this.totalSlides) {
                this.showSlide(slideNumber);
            }
        }

        preloadAdjacentSlides() {
            // Preload next slide
            if (this.currentSlide < this.totalSlides) {
                const nextSlide = this.slides[this.currentSlide];
                if (nextSlide && !nextSlide.dataset.loaded) {
                    this.loadSlideContent(this.currentSlide + 1, nextSlide);
                }
            }
            
            // Preload previous slide
            if (this.currentSlide > 1) {
                const prevSlide = this.slides[this.currentSlide - 2];
                if (prevSlide && !prevSlide.dataset.loaded) {
                    this.loadSlideContent(this.currentSlide - 1, prevSlide);
                }
            }
        }

        toggleFullscreen() {
            if (!document.fullscreenElement) {
                document.documentElement.requestFullscreen().catch(err => {
                    console.error('Error attempting to enable fullscreen:', err);
                });
            } else {
                document.exitFullscreen();
            }
        }

        handleResize() {
            // Adjust font sizes or layout if needed
            const width = window.innerWidth;
            const height = window.innerHeight;
            
            // You can add responsive adjustments here
            if (width < 768) {
                document.documentElement.style.setProperty('--base-font-size', '16px');
            } else {
                document.documentElement.style.setProperty('--base-font-size', '20px');
            }
        }

        emitSlideChangeEvent(slideNumber) {
            const event = new CustomEvent('slidechange', {
                detail: {
                    current: slideNumber,
                    total: this.totalSlides,
                    previous: this.currentSlide
                }
            });
            document.dispatchEvent(event);
        }

        // Public API
        getCurrentSlide() {
            return this.currentSlide;
        }

        getTotalSlides() {
            return this.totalSlides;
        }

        goToSlide(slideNumber) {
            this.showSlide(slideNumber);
        }
    }

    // Initialize theme when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            window.SliCLITheme = new DefaultTheme();
        });
    } else {
        window.SliCLITheme = new DefaultTheme();
    }

    // Export for use in other scripts
    window.DefaultTheme = DefaultTheme;
})();