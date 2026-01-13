#include "upgrade_query.h"
#include <iostream>
#include <iomanip>
#include <string>
#include <nlohmann/json.hpp>

int main(int argc, char **argv)
{
    std::string sourcelist = "";
    std::string sourcepart = "";
    bool outputJson = false;
    bool allowDowngrades = false;

    // Support customizing sourcelist path via command line arguments
    for (int i = 1; i < argc; i++) {
        std::string arg = argv[i];
        if (arg == "--sourcelist" && i + 1 < argc) {
            sourcelist = argv[++i];
        } else if (arg == "--sourceparts" && i + 1 < argc) {
            sourcepart = argv[++i];
        } else if (arg == "-j" || arg == "--json") {
            outputJson = true;
        } else if (arg == "--allow-downgrades") {
            allowDowngrades = true;
        } else if (arg == "--help" || arg == "-h") {
            std::cout << "Usage: " << argv[0] << " [options]\n"
                      << "Options:\n"
                      << "  --sourcelist <file>   Specify custom sources.list file path\n"
                      << "  --sourceparts <dir>   Specify custom sources.list.d directory path\n"
                      << "  -j, --json           Output results in JSON format\n"
                      << "  --allow-downgrades   Allow downgrade installation\n"
                      << "  --help, -h           Show this help message\n";
            return 0;
        }
    }

    std::vector<UpgradePackage> packages =
            GetUpgradePackages(sourcelist, sourcepart, allowDowngrades);

    for (const auto &pkgItem : packages) {
        if (!pkgItem.Valid()) {
            std::cerr << "Invalid package: " << pkgItem.Name << std::endl;
            return 1;
        }
    }

    if (outputJson) {
        nlohmann::json json_array = nlohmann::json::array();

        for (const auto &pkg : packages) {
            json_array.push_back({ { "name", pkg.Name },
                                   { "installed_version", pkg.InstalledVersion },
                                   { "candidate_version", pkg.CandidateVersion },
                                   { "architecture", pkg.Architecture },
                                   { "codename", pkg.Codename },
                                   { "component", pkg.Component },
                                   { "site", pkg.Site },
                                   { "filename", pkg.Filename },
                                   { "size", pkg.Size },
                                   { "installed_size", pkg.InstalledSize },
                                   { "hash", pkg.Hash },
                                   { "uri", pkg.Uri } });
        }

        std::cout << json_array.dump() << '\n';
    } else {
        std::cout << "\n=== Retrieved Package Information ===\n"
                  << "Total " << packages.size() << " packages found\n";

        for (const auto &pkg : packages) {
            std::cout << "\nname: " << pkg.Name << "\n"
                      << "candidate_version: " << pkg.CandidateVersion << "\n";

            if (!pkg.InstalledVersion.empty()) {
                std::cout << "installed_version: " << pkg.InstalledVersion << "\n";
            }

            std::cout << "architecture: " << pkg.Architecture << "\n"
                      << "codename: " << pkg.Codename << "\n"
                      << "component: " << pkg.Component << "\n"
                      << "site: " << pkg.Site << "\n"
                      << "filename: " << pkg.Filename << "\n"
                      << "size: " << std::fixed << std::setprecision(2)
                      << pkg.Size / 1024.0 / 1024.0 << " MB\n"
                      << "installed_size: " << std::fixed << std::setprecision(2)
                      << pkg.InstalledSize / 1024.0 / 1024.0 << " MB\n"
                      << "hash: " << pkg.Hash << "\n"
                      << "uri: " << pkg.Uri << "\n";
        }
    }

    return 0;
}
