#pragma once

#include <vector>
#include <string>
#include <cstdint>

class UpgradePackage {
public:
    std::string Name;
    std::string InstalledVersion;
    std::string CandidateVersion;
    std::string Architecture;
    std::string Codename;
    std::string Component;
    std::string Site;
    std::string Filename;
    uint64_t Size = 0;
    uint64_t InstalledSize = 0;
    std::string Hash;

    bool Valid() const;
};

std::vector<UpgradePackage> GetUpgradePackages(const std::string &sourcelist, const std::string &sourceparts, bool allow_downgrades = false);
