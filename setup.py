from setuptools import setup, find_packages

setup(
    name='pulsevault',
    version='2.0.0',
    description='Next-generation encrypted file vault by DNSPulse.',
    author='DNSPulse (z3r0s)',
    author_email='contact@dnspulse.com',
    url='https://github.com/z3r0s/pulsevault',
    packages=find_packages(),
    py_modules=['main'],
    install_requires=[
        'cryptography>=41.0.0',
        'customtkinter>=5.2.0',
        'tkinterdnd2>=0.3.0',
    ],
    entry_points={
        'console_scripts': [
            'pulsevault=main:main',
        ],
    },
    classifiers=[
        'Programming Language :: Python :: 3',
        'License :: OSI Approved :: MIT License',
        'Operating System :: POSIX :: Linux',
        'Operating System :: Microsoft :: Windows',
    ],
    python_requires='>=3.8',
)
