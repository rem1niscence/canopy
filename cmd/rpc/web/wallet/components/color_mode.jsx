import { useEffect, useState } from 'react';

const DarkModeToggle = () => {
    const [theme, setTheme] = useState(() => {
        const storedTheme = localStorage.getItem('bsTheme');
        return storedTheme || 'light';
    });

    useEffect(() => {
        // Set the data-bs-theme attribute on the html element *and* the nav
        document.documentElement.setAttribute('data-bs-theme', theme);
        const navElement = document.getElementById('nav-bar'); // Get the nav element
        if (navElement) {
            navElement.setAttribute('data-bs-theme', theme);
            if (theme === 'dark') {
                navElement.classList.replace("navbar-light", "navbar-dark");
            } else {
                navElement.classList.replace("navbar-dark", "navbar-light");
            }
        }

        localStorage.setItem('bsTheme', theme);

        const element = document.querySelector("#colorSwitchLabel");
        if (element) {
            if (theme === 'dark') {
                element.classList.replace("bi-sun-fill", "bi-moon-stars-fill");
            } else {
                element.classList.replace("bi-moon-stars-fill", "bi-sun-fill");
            }
        }

        const element = document.querySelector("footer");
        if (element) {
            if (theme === 'dark') {
                element.classList.replace("footer-light", "footer-dark");
            } else {
                element.classList.replace("footer-dark", "footer-light");
            }
        }
    }, [theme]);

    const handleChange = (event) => {
        setTheme(event.target.checked ? 'dark' : 'light');
    };

    return (
        <div className="form-check form-switch color-mode"> {/* Added color-mode class */}
            <input
                className="form-check-input"
                type="checkbox"
                id="darkModeSwitch"
                checked={theme === 'dark'}
                onChange={handleChange}
            />
            <label className="form-check-label" htmlFor="darkModeSwitch" id="colorSwitchLabel">
                <i className={`bi bi-${theme === 'dark' ? 'moon-stars-fill' : 'sun-fill'}`}></i>
            </label>
        </div>
    );
};

export default DarkModeToggle;